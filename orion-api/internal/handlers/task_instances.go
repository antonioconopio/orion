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

func NewTaskInstanceHandler(db *pgxpool.Pool) *TaskInstanceHandler {
	return &TaskInstanceHandler{DB: db}
}

// List godoc
// @Summary      List task instances
// @Tags         task-instances
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Success      200  {array}  models.TaskInstance
// @Failure      400  {string}  string
// @Failure      500  {string}  string
// @Router       /dags/{dagId}/task-instances [get]
func (h *TaskInstanceHandler) List(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("runId")
	id, err := uuid.Parse(runID)
	if err != nil {
		http.Error(w, "invalid run id", http.StatusBadRequest)
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT id, run_id, task_id, status
		FROM task_instances
		WHERE run_id = $1
	`, id)
	if err != nil {
		http.Error(w, "failed to query task instances", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	taskInstances := []models.TaskInstance{}
	for rows.Next() {
		var ti models.TaskInstance
		err := rows.Scan(&ti.ID, &ti.RunID, &ti.TaskID, &ti.Status)
		if err != nil {
			http.Error(w, "failed to scan task instance", http.StatusInternalServerError)
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
// @Failure      400  {string}  string
// @Failure      404  {string}  string
// @Failure      500  {string}  string
// @Router       /task-instances/{id} [get]
func (h *TaskInstanceHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		http.Error(w, "invalid task instance id", http.StatusBadRequest)
		return
	}

	row := h.DB.QueryRow(r.Context(), `
		SELECT id, run_id, task_id, status
		FROM task_instances
		WHERE id = $1
	`, id)

	var ti models.TaskInstance
	err = row.Scan(&ti.ID, &ti.RunID, &ti.TaskID, &ti.Status)
	if err == pgx.ErrNoRows {
		http.Error(w, "task instance not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "failed to query task instance", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, ti)
}