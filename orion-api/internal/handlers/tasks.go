package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
)

type TaskHandler struct {
	DB *pgxpool.Pool
}

func NewTaskHandler (db *pgxpool.Pool) *TaskHandler {
	return &TaskHandler{DB: db}
}

// List handles GET /dags/{dagId}/tasks — returns every task belonging
// to one DAG.
//
// List godoc
// @Summary      List tasks for a DAG
// @Tags         tasks
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Success      200  {array}  models.Task
// @Failure      400  {string}  string
// @Failure      500  {string}  string
// @Router       /dags/{dagId}/tasks [get]
func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	dagId := r.PathValue("dagId")
	id, err := uuid.Parse(dagId)
	if err != nil {
		http.Error(w, "invalid dag id", http.StatusBadRequest)
		return
	}

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

	tasks := []models.Task{}
	for rows.Next() {
		var task models.Task
		err := rows.Scan(&task.ID, &task.DAGID, &task.Name, &task.TaskType, &task.Config, &task.CreatedAt)
		if err != nil {
			http.Error(w, "failed to scan task", http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, task)
	}
	writeJSON(w, http.StatusOK, tasks)
}

// Get handles GET /tasks/{id} — returns one task by its own UUID.
// Same shape as DAGHandler.Get — copy that pattern and swap the table
// and struct.
//
// Get godoc
// @Summary      Get a task
// @Tags         tasks
// @Produce      json
// @Param        id  path  string  true  "Task ID"
// @Success      200  {object}  models.Task
// @Failure      400  {string}  string
// @Failure      404  {object}  map[string]string
// @Failure      500  {string}  string
// @Router       /tasks/{id} [get]
func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return
	}
	
	var task models.Task
	err = h.DB.QueryRow(r.Context(), `
		SELECT id, dag_id, name, task_type, config, created_at
		FROM tasks
		WHERE id = $1
	`, id).Scan(&task.ID, &task.DAGID, &task.Name, &task.TaskType, &task.Config, &task.CreatedAt)
	
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	
	if err != nil {
		http.Error(w, "failed to query task", http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, task)
}

// Create handles POST /dags/{dagId}/tasks — creates a task under a DAG.
//
// Extra wrinkle vs. DAGHandler.Create: dag_id comes from the URL, not
// the JSON body (a task always belongs to whatever DAG the URL names).
// Everything else — decode body, INSERT ... RETURNING, scan result — is
// the same shape.
//
// Create godoc
// @Summary      Create a task under a DAG
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Param        task  body  models.CreateTaskRequest  true  "Task to create"
// @Success      201  {object}  models.Task
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{dagId}/tasks [post]
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	dagId := r.PathValue("dagId")
	id, err := uuid.Parse(dagId)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	var task models.Task
	err = h.DB.QueryRow(r.Context(), `
		INSERT INTO tasks (dag_id, name, task_type, config)
		VALUES ($1, $2, $3, $4)
		RETURNING id, dag_id, name, task_type, config, created_at
	`, id, req.Name, req.TaskType, req.Config).Scan(&task.ID, &task.DAGID, &task.Name, &task.TaskType, &task.Config, &task.CreatedAt)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

// Delete godoc
// @Summary      Delete a task
// @Tags         tasks
// @Produce      json
// @Param        id  path  string  true  "Task ID"
// @Success      204  "No Content"
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /tasks/{id} [delete]
func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	cmdTag, err := h.DB.Exec(r.Context(), `
		DELETE FROM tasks
		WHERE id = $1
	`, id)

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete task")
		return
	}

	if cmdTag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Update godoc
// @Summary      Update a task
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "Task ID"
// @Param        task  body  models.CreateTaskRequest  true  "New task values"
// @Success      200  {object}  models.Task
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /tasks/{id} [put]
func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid task id")
		return
	}

	var req models.CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var task models.Task
	err = h.DB.QueryRow(r.Context(), `
		UPDATE tasks
		SET name = $1, task_type = $2, config = $3
		WHERE id = $4
		RETURNING id, dag_id, name, task_type, config, created_at
	`, req.Name, req.TaskType, req.Config, id).Scan(&task.ID, &task.DAGID, &task.Name, &task.TaskType, &task.Config, &task.CreatedAt)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update task")
		return
	}

	writeJSON(w, http.StatusOK, task)
}