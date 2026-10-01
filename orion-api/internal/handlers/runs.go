package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
	"orion-api/internal/queue"
)

type RunHandler struct {
	DB *pgxpool.Pool
	Queue *queue.Stream
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
// @Failure      400  {string}  string
// @Failure      500  {string}  string
// @Router       /dags/{dagId}/runs [get]
func (h *RunHandler) List(w http.ResponseWriter, r *http.Request) {
	dagId := r.PathValue("dagId")
	id, err := uuid.Parse(dagId)
	if err != nil {
		http.Error(w, "invalid dag id", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT id, dag_id, status, created_at
		FROM runs
		WHERE dag_id = $1
	`, id)
	if err != nil {
		http.Error(w, "failed to query runs", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	runs := []models.Run{}
	for rows.Next() {
		var run models.Run
		err := rows.Scan(&run.ID, &run.DAGID, &run.Status, &run.CreatedAt)
		if err != nil {
			http.Error(w, "failed to scan run", http.StatusInternalServerError)
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
// @Failure      400  {string}  string
// @Failure      404  {string}  string
// @Failure      500  {string}  string
// @Router       /runs/{id} [get]
func (h *RunHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	var run models.Run
	err = h.DB.QueryRow(r.Context(), `
		SELECT id, dag_id, status, created_at
		FROM runs
		WHERE id = $1
	`, id).Scan(&run.ID, &run.DAGID, &run.Status, &run.CreatedAt)

	if err == pgx.ErrNoRows {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "failed to query run", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, run)
}

// Trigger godoc
// @Summary      Trigger a DAG run
// @Tags         runs
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Success      201  {object}  models.Run
// @Failure      400  {string}  string
// @Failure      500  {string}  string
// @Router       /dags/{dagId}/runs [post]
func (h *RunHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	dagId := r.PathValue("dagId")
	id, err := uuid.Parse(dagId)
	if err != nil {
		http.Error(w, "invalid dag id", http.StatusBadRequest)
		return
	}

	var run models.Run
	err = h.DB.QueryRow(r.Context(), `
		INSERT INTO runs (dag_id, status)
		VALUES ($1, 'queued')
		RETURNING id, dag_id, status, created_at
	`, id).Scan(&run.ID, &run.DAGID, &run.Status, &run.CreatedAt)

	if err != nil {
		http.Error(w, "failed to create run", http.StatusInternalServerError)
		return
	}

	var tasks []models.Task
	rows, err := h.DB.Query(r.Context(), `
		SELECT id, dag_id, name, task_type, config, created_at
		FROM tasks
		WHERE dag_id = $1
	`, id)
	if err != nil {
		http.Error(w, "failed to query tasks", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var task models.Task
		err := rows.Scan(&task.ID, &task.DAGID, &task.Name, &task.TaskType, &task.Config, &task.CreatedAt)
		if err != nil {
			http.Error(w, "failed to scan task", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}

	var task_instances []models.TaskInstance
	for _, task := range tasks {
		var taskInstance models.TaskInstance
		err = h.DB.QueryRow(r.Context(), `
			INSERT INTO task_instances (run_id, task_id, status)
			VALUES ($1, $2, 'pending')
			RETURNING id, run_id, task_id, status
		`, run.ID, task.ID).Scan(&taskInstance.ID, &taskInstance.RunID, &taskInstance.TaskID, &taskInstance.Status)
		if err != nil {
			http.Error(w, "failed to create task instance", http.StatusInternalServerError)
			return
		}
		task_instances = append(task_instances, taskInstance)
	}

	var readySet []uuid.UUID
	for _, task:= range tasks {
		var id uuid.UUID
		err = h.DB.QueryRow(r.Context(), `
			SELECT id
			FROM task_instances
			WHERE run_id = $1 AND NOT EXISTS (
				SELECT 1
				FROM task_dependencies td
				WHERE td.task_id = $2)
		`, run.ID, task.ID).Scan(&id)

		if err != nil {
			http.Error(w, "failed to query ready tasks", http.StatusInternalServerError)
			return
		}
		readySet = append(readySet, id)
	}

	var enqueResults []string
	for _, id := range readySet {
		msg, err := h.Queue.Enqueue(r.Context(), id.String())

		if err != nil {
			http.Error(w, "failed to enqueue tasks", http.StatusInternalServerError)
			return
		}
		enqueResults = append(enqueResults, msg)
	}
	
	writeJSON(w, http.StatusCreated, TriggerRunResponse{Run: run, TaskInstances: task_instances, Enqueued: enqueResults})
}