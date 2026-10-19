package handlers

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
)

type TaskInstanceHandler struct {
	DB *pgxpool.Pool
}

const taskInstanceCols = `id, run_id, task_id, status, attempt, worker_id, started_at, finished_at, logs, output`

// scanTaskInstance reads taskInstanceCols from a pgx.Row or pgx.Rows.
func scanTaskInstance(row pgx.Row, ti *models.TaskInstance) error {
	return row.Scan(&ti.ID, &ti.RunID, &ti.TaskID, &ti.Status, &ti.Attempt, &ti.WorkerID,
		&ti.StartedAt, &ti.FinishedAt, &ti.Logs, &ti.Output)
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
	runID := r.PathValue("runId")
	id, err := uuid.Parse(runID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid run id")
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT `+taskInstanceCols+`
		FROM task_instances
		WHERE run_id = $1
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query task instances")
		return
	}
	defer rows.Close()

	taskInstances := []models.TaskInstance{}
	for rows.Next() {
		var ti models.TaskInstance
		err := scanTaskInstance(rows, &ti)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan task instance")
			return
		}
		taskInstances = append(taskInstances, ti)
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
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task instance id")
		return
	}

	row := h.DB.QueryRow(r.Context(), `
		SELECT `+taskInstanceCols+`
		FROM task_instances
		WHERE id = $1
	`, id)

	var ti models.TaskInstance
	err = scanTaskInstance(row, &ti)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "task instance not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query task instance")
		return
	}

	writeJSON(w, http.StatusOK, ti)
}