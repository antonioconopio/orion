package handlers

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
	"orion-api/internal/queue"
)

type RunHandler struct {
	DB    *pgxpool.Pool
	Queue *queue.Stream
}

// runSelect reads a run with its DAG name and task counts. Callers append a WHERE / ORDER BY.
const runSelect = `
	SELECT r.id, r.dag_id, d.name, r.number, r.status, r.triggered_by,
	       r.started_at, r.finished_at, r.created_at,
	       COALESCE(c.total, 0), COALESCE(c.done, 0)
	FROM runs r
	JOIN dags d ON d.id = r.dag_id
	LEFT JOIN LATERAL (
		SELECT count(*) AS total,
		       count(*) FILTER (WHERE status IN ('success', 'failed', 'skipped')) AS done
		FROM task_instances WHERE run_id = r.id
	) c ON true`

// scanRun reads the columns of runSelect from a pgx.Row or pgx.Rows.
func scanRun(row pgx.Row, run *models.Run) error {
	return row.Scan(&run.ID, &run.DAGID, &run.DAGName, &run.Number, &run.Status, &run.TriggeredBy,
		&run.StartedAt, &run.FinishedAt, &run.CreatedAt, &run.TasksTotal, &run.TasksDone)
}

func NewRunHandler(db *pgxpool.Pool, queue *queue.Stream) *RunHandler {
	return &RunHandler{DB: db, Queue: queue}
}

type TriggerRunResponse struct {
	Run           models.Run            `json:"run"`
	TaskInstances []models.TaskInstance `json:"task_instances"`
	Enqueued      []string              `json:"enqueued"`
}

// queryRuns runs a runSelect-based query and writes the result as JSON.
func (h *RunHandler) queryRuns(w http.ResponseWriter, r *http.Request, query string, args ...any) {
	rows, err := h.DB.Query(r.Context(), query, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query runs")
		return
	}
	defer rows.Close()

	runs := []models.Run{}
	for rows.Next() {
		var run models.Run
		if err := scanRun(rows, &run); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan run")
			return
		}
		runs = append(runs, run)
	}
	writeJSON(w, http.StatusOK, runs)
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
	id, err := uuid.Parse(r.PathValue("dagId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}
	h.queryRuns(w, r, runSelect+` WHERE r.dag_id = $1 ORDER BY r.created_at DESC`, id)
}

var validRunStatuses = map[string]bool{"queued": true, "running": true, "success": true, "failed": true, "cancelled": true}

// ListAll godoc
// @Summary      List runs across all DAGs, newest first
// @Tags         runs
// @Produce      json
// @Param        limit   query  int     false  "Max rows (default 50, max 200)"
// @Param        status  query  string  false  "Only runs with this status"
// @Param        dag_id  query  string  false  "Only runs of this DAG"
// @Success      200  {array}  models.Run
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /runs [get]
func (h *RunHandler) ListAll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := 50
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			writeError(w, http.StatusBadRequest, "limit must be between 1 and 200")
			return
		}
		limit = n
	}

	var status, dagID any // nil = no filter
	if v := q.Get("status"); v != "" {
		if !validRunStatuses[v] {
			writeError(w, http.StatusBadRequest, "invalid status")
			return
		}
		status = v
	}
	if v := q.Get("dag_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid dag id")
			return
		}
		dagID = id
	}

	h.queryRuns(w, r, runSelect+`
		WHERE ($1::text IS NULL OR r.status = $1)
		  AND ($2::uuid IS NULL OR r.dag_id = $2)
		ORDER BY r.created_at DESC
		LIMIT $3`, status, dagID, limit)
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
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	var run models.Run
	err = scanRun(h.DB.QueryRow(r.Context(), runSelect+` WHERE r.id = $1`, id), &run)
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

	// The DAG's counter hands out the run number (and locks the row, so numbers never collide).
	var number int
	err = tx.QueryRow(r.Context(), `
		UPDATE dags SET run_counter = run_counter + 1 WHERE id = $1 RETURNING run_counter
	`, dagID).Scan(&number)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "dag not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to allocate run number")
		return
	}

	var runID uuid.UUID
	err = tx.QueryRow(r.Context(), `
		INSERT INTO runs (dag_id, number, status) VALUES ($1, $2, 'queued') RETURNING id
	`, dagID, number).Scan(&runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create run")
		return
	}

	// One instance per task. Tasks with no dependencies start 'queued' (the worker only
	// starts queued instances); the rest wait as 'pending' until their parents succeed.
	tag, err := tx.Exec(r.Context(), `
		INSERT INTO task_instances (run_id, task_id, status, queued_at)
		SELECT $1::uuid, t.id,
		       CASE WHEN t.has_deps THEN 'pending' ELSE 'queued' END,
		       CASE WHEN t.has_deps THEN NULL ELSE NOW() END
		FROM (
			SELECT id, EXISTS (SELECT 1 FROM task_dependencies td WHERE td.task_id = tasks.id) AS has_deps
			FROM tasks WHERE dag_id = $2
		) t
	`, runID, dagID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task instances")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusBadRequest, "dag has no tasks") // an empty run would stay queued forever
		return
	}

	var run models.Run
	if err := scanRun(tx.QueryRow(r.Context(), runSelect+` WHERE r.id = $1`, runID), &run); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read run")
		return
	}
	taskInstances, err := queryTaskInstances(r, tx, `WHERE ti.run_id = $1`, runID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read task instances")
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
