package handlers

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
	"orion-api/internal/queue"
)

type RunHandler struct {
	DB *pgxpool.Pool
	Queue *queue.Stream
}

const runCols = `id, dag_id, status, triggered_by, started_at, finished_at, created_at`

// scanRun reads runCols from a pgx.Row or pgx.Rows.
func scanRun(row pgx.Row, run *models.Run) error {
	return row.Scan(&run.ID, &run.DAGID, &run.Status, &run.TriggeredBy, &run.StartedAt, &run.FinishedAt, &run.CreatedAt)
}

func NewRunHandler(db *pgxpool.Pool, queue *queue.Stream) *RunHandler {
	return &RunHandler{DB: db, Queue: queue}
}

type TriggerRunResponse struct {
	Run           models.Run            `json:"run"`
	TaskInstances []models.TaskInstance `json:"task_instances"`
	Enqueued      []string              `json:"enqueued"`
}

// List godoc
// @Summary      List runs for a DAG
// @Tags         runs
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Success      200  {array}  models.Run
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{dagId}/runs [get]
func (h *RunHandler) List(w http.ResponseWriter, r *http.Request) {
	dagId := r.PathValue("dagId")
	id, err := uuid.Parse(dagId)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT `+runCols+`
		FROM runs
		WHERE dag_id = $1
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query runs")
		return
	}
	defer rows.Close()

	runs := []models.Run{}
	for rows.Next() {
		var run models.Run
		err := scanRun(rows, &run)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan run")
			return
		}
		runs = append(runs, run)
	}
	writeJSON(w, http.StatusOK, runs)
}

// Get godoc
// @Summary      Get a run
// @Tags         runs
// @Produce      json
// @Param        id  path  string  true  "Run ID"
// @Success      200  {object}  models.Run
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /runs/{id} [get]
func (h *RunHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	var run models.Run
	err = scanRun(h.DB.QueryRow(r.Context(), `
		SELECT `+runCols+`
		FROM runs
		WHERE id = $1
	`, id), &run)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "run not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query run")
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// Trigger godoc
// @Summary      Trigger a DAG run
// @Description  Creates the run and one task instance per task, then enqueues the tasks that have no dependencies. If Redis is down after the run is created, the roots stay queued and a worker's sweeper re-enqueues them.
// @Tags         runs
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Success      201  {object}  TriggerRunResponse
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{dagId}/runs [post]
func (h *RunHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	dagID, err := uuid.Parse(r.PathValue("dagId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context())

	var run models.Run
	err = scanRun(tx.QueryRow(r.Context(), `
		INSERT INTO runs (dag_id, status)
		VALUES ($1, 'queued')
		RETURNING `+runCols+`
	`, dagID), &run)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign key: no such dag
		writeError(w, http.StatusNotFound, "dag not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create run")
		return
	}

	// One instance per task. Tasks with no dependencies start 'queued' (the worker only
	// starts queued instances); the rest wait as 'pending' until their parents succeed.
	rows, err := tx.Query(r.Context(), `
		INSERT INTO task_instances (run_id, task_id, status, queued_at)
		SELECT $1::uuid, t.id,
		       CASE WHEN t.has_deps THEN 'pending' ELSE 'queued' END,
		       CASE WHEN t.has_deps THEN NULL ELSE NOW() END
		FROM (
			SELECT id, EXISTS (SELECT 1 FROM task_dependencies td WHERE td.task_id = tasks.id) AS has_deps
			FROM tasks WHERE dag_id = $2
		) t
		RETURNING `+taskInstanceCols+`
	`, run.ID, dagID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task instances")
		return
	}
	taskInstances := []models.TaskInstance{}
	for rows.Next() {
		var ti models.TaskInstance
		if err := scanTaskInstance(rows, &ti); err != nil {
			rows.Close()
			writeError(w, http.StatusInternalServerError, "failed to scan task instance")
			return
		}
		taskInstances = append(taskInstances, ti)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task instances")
		return
	}
	if len(taskInstances) == 0 {
		writeError(w, http.StatusBadRequest, "dag has no tasks") // an empty run would stay queued forever
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit run")
		return
	}

	// Enqueue only after the commit, so a worker never sees a message for a row that is not there yet.
	enqueued := []string{}
	for _, ti := range taskInstances {
		if ti.Status != "queued" {
			continue
		}
		msg, err := h.Queue.Enqueue(r.Context(), ti.ID.String())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "run "+run.ID.String()+" was created but could not be enqueued; queued tasks will be retried by the worker sweeper")
			return
		}
		enqueued = append(enqueued, msg)
	}

	writeJSON(w, http.StatusCreated, TriggerRunResponse{Run: run, TaskInstances: taskInstances, Enqueued: enqueued})
}
