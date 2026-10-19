package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"orion-api/internal/models"
)

// WorkerKeyPrefix is where each worker writes its heartbeat: <prefix><worker id>, with a short TTL.
const WorkerKeyPrefix = "orion:worker:"

type WorkerHandler struct {
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Prefix string
}

func NewWorkerHandler(db *pgxpool.Pool, rdb *redis.Client) *WorkerHandler {
	return &WorkerHandler{DB: db, Redis: rdb, Prefix: WorkerKeyPrefix}
}

// heartbeat mirrors the JSON the Python worker writes.
type heartbeat struct {
	ID                string    `json:"id"`
	Host              string    `json:"host"`
	PID               int       `json:"pid"`
	StartedAt         time.Time `json:"started_at"`
	Status            string    `json:"status"`
	CurrentInstanceID *string   `json:"current_instance_id"`
}

// List godoc
// @Summary      List live workers
// @Description  Read from worker heartbeats in Redis; a worker that stops heartbeating (about 15 seconds) drops out of the list.
// @Tags         workers
// @Produce      json
// @Success      200  {array}  models.Worker
// @Failure      500  {object}  map[string]string
// @Router       /workers [get]
func (h *WorkerHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var keys []string
	iter := h.Redis.Scan(ctx, 0, h.Prefix+"*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list workers")
		return
	}

	workers := []models.Worker{}
	if len(keys) == 0 {
		writeJSON(w, http.StatusOK, workers)
		return
	}
	values, err := h.Redis.MGet(ctx, keys...).Result()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read workers")
		return
	}

	var busyIDs []string
	beats := []heartbeat{}
	for _, v := range values {
		raw, ok := v.(string) // nil: the key expired between SCAN and MGET
		if !ok {
			continue
		}
		var hb heartbeat
		if json.Unmarshal([]byte(raw), &hb) != nil {
			continue
		}
		beats = append(beats, hb)
		if hb.CurrentInstanceID != nil {
			busyIDs = append(busyIDs, *hb.CurrentInstanceID)
		}
	}

	names, err := h.currentTasks(ctx, busyIDs)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to look up current tasks")
		return
	}
	for _, hb := range beats {
		wk := models.Worker{ID: hb.ID, Host: hb.Host, PID: hb.PID, StartedAt: hb.StartedAt, Status: hb.Status}
		if hb.CurrentInstanceID != nil {
			if cur, ok := names[*hb.CurrentInstanceID]; ok {
				name, runID := cur[0], cur[1]
				wk.CurrentTask, wk.CurrentRunID = &name, &runID
			}
		}
		workers = append(workers, wk)
	}
	sort.Slice(workers, func(i, j int) bool { return workers[i].ID < workers[j].ID })
	writeJSON(w, http.StatusOK, workers)
}

// currentTasks maps task instance id -> ["dag / task", run id] in one query.
func (h *WorkerHandler) currentTasks(ctx context.Context, instanceIDs []string) (map[string][2]string, error) {
	out := map[string][2]string{}
	if len(instanceIDs) == 0 {
		return out, nil
	}
	rows, err := h.DB.Query(ctx, `
		SELECT ti.id::text, d.name || ' / ' || t.name, ti.run_id::text
		FROM task_instances ti
		JOIN tasks t ON t.id = ti.task_id
		JOIN dags d ON d.id = t.dag_id
		WHERE ti.id::text = ANY($1)
	`, instanceIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, name, runID string
		if err := rows.Scan(&id, &name, &runID); err != nil {
			return nil, err
		}
		out[id] = [2]string{name, runID}
	}
	return out, rows.Err()
}
