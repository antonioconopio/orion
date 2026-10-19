package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
)

type TaskInstanceHandler struct {
	DB *pgxpool.Pool
}

// taskInstanceSelect reads an instance with its task's name. Callers append a WHERE / ORDER BY.
const taskInstanceSelect = `
	SELECT ti.id, ti.run_id, ti.task_id, t.name, ti.status, ti.attempt, ti.worker_id,
	       ti.started_at, ti.finished_at, ti.logs, ti.output
	FROM task_instances ti
	JOIN tasks t ON t.id = ti.task_id`

// scanTaskInstance reads the columns of taskInstanceSelect from a pgx.Row or pgx.Rows.
func scanTaskInstance(row pgx.Row, ti *models.TaskInstance) error {
	return row.Scan(&ti.ID, &ti.RunID, &ti.TaskID, &ti.TaskName, &ti.Status, &ti.Attempt, &ti.WorkerID,
		&ti.StartedAt, &ti.FinishedAt, &ti.Logs, &ti.Output)
}

// querier is satisfied by both *pgxpool.Pool and pgx.Tx.
type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// queryTaskInstances returns instances matching where (e.g. "WHERE ti.run_id = $1"), ordered by task name.
func queryTaskInstances(r *http.Request, q querier, where string, args ...any) ([]models.TaskInstance, error) {
	rows, err := q.Query(r.Context(), taskInstanceSelect+" "+where+" ORDER BY t.name", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []models.TaskInstance{}
	for rows.Next() {
		var ti models.TaskInstance
		if err := scanTaskInstance(rows, &ti); err != nil {
			return nil, err
		}
		out = append(out, ti)
	}
	return out, rows.Err()
}

func NewTaskInstanceHandler(db *pgxpool.Pool) *TaskInstanceHandler {
	return &TaskInstanceHandler{DB: db}
}

// List godoc
// @Summary      List task instances
// @Tags         task-instances
// @Produce      json
// @Param        runId  path  string  true  "Run ID"
// @Success      200  {array}  models.TaskInstance
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /runs/{runId}/task-instances [get]
func (h *TaskInstanceHandler) List(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("runId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	taskInstances, err := queryTaskInstances(r, h.DB, `WHERE ti.run_id = $1`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query task instances")
		return
	}
	writeJSON(w, http.StatusOK, taskInstances)
}

// Get godoc
// @Summary      Get a task instance
// @Tags         task-instances
// @Produce      json
// @Param        id  path  string  true  "Task instance ID"
// @Success      200  {object}  models.TaskInstance
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /task-instances/{id} [get]
func (h *TaskInstanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task instance id")
		return
	}

	var ti models.TaskInstance
	err = scanTaskInstance(h.DB.QueryRow(r.Context(), taskInstanceSelect+` WHERE ti.id = $1`, id), &ti)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "task instance not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query task instance")
		return
	}

	writeJSON(w, http.StatusOK, ti)
}
