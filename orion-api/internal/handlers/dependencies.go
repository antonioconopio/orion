package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
)

type DependencyHandler struct {
	DB *pgxpool.Pool
}

func NewDependencyHandler(db *pgxpool.Pool) *DependencyHandler {
	return &DependencyHandler{DB: db}
}

// List godoc
// @Summary      List dependencies for a DAG
// @Tags         dependencies
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Success      200  {array}  models.TaskDependency
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{dagId}/dependencies [get]
func (h *DependencyHandler) List(w http.ResponseWriter, r *http.Request) {
	dagID, err := uuid.Parse(r.PathValue("dagId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT td.task_id, td.depends_on_task_id
		FROM task_dependencies td
		JOIN tasks t ON t.id = td.task_id
		WHERE t.dag_id = $1
	`, dagID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query dependencies")
		return
	}
	defer rows.Close()

	deps := []models.TaskDependency{}
	for rows.Next() {
		var d models.TaskDependency
		if err := rows.Scan(&d.TaskID, &d.DependsOnTaskID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan dependency")
			return
		}
		deps = append(deps, d)
	}
	writeJSON(w, http.StatusOK, deps)
}

// Create godoc
// @Summary      Make one task depend on another
// @Description  task_id will only run after depends_on_task_id succeeds. Both tasks must belong to the DAG; self-dependencies, duplicates and cycles are rejected.
// @Tags         dependencies
// @Accept       json
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Param        dependency  body  models.CreateTaskDependencyRequest  true  "Edge to add"
// @Success      201  {object}  models.TaskDependency
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      409  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{dagId}/dependencies [post]
func (h *DependencyHandler) Create(w http.ResponseWriter, r *http.Request) {
	dagID, err := uuid.Parse(r.PathValue("dagId"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	var req models.CreateTaskDependencyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.TaskID == req.DependsOnTaskID {
		writeError(w, http.StatusBadRequest, "a task cannot depend on itself")
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to begin transaction")
		return
	}
	defer tx.Rollback(r.Context())

	// One edge insert at a time per DAG: two concurrent requests could otherwise each
	// pass the cycle check and together create a cycle.
	if _, err := tx.Exec(r.Context(), `SELECT pg_advisory_xact_lock(hashtext($1::text))`, dagID.String()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to lock dag")
		return
	}

	var inDAG int
	err = tx.QueryRow(r.Context(), `
		SELECT count(*) FROM tasks WHERE dag_id = $1 AND id IN ($2, $3)
	`, dagID, req.TaskID, req.DependsOnTaskID).Scan(&inDAG)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to look up tasks")
		return
	}
	if inDAG != 2 {
		writeError(w, http.StatusNotFound, "both tasks must exist in this dag")
		return
	}

	// task_id -> depends_on is a cycle if depends_on already depends (transitively) on task_id.
	var cycle bool
	err = tx.QueryRow(r.Context(), `
		WITH RECURSIVE upstream AS (
			SELECT depends_on_task_id AS id FROM task_dependencies WHERE task_id = $1
			UNION
			SELECT td.depends_on_task_id FROM task_dependencies td JOIN upstream u ON td.task_id = u.id
		)
		SELECT EXISTS (SELECT 1 FROM upstream WHERE id = $2)
	`, req.DependsOnTaskID, req.TaskID).Scan(&cycle)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check for cycles")
		return
	}
	if cycle {
		writeError(w, http.StatusConflict, "dependency would create a cycle")
		return
	}

	_, err = tx.Exec(r.Context(), `
		INSERT INTO task_dependencies (task_id, depends_on_task_id) VALUES ($1, $2)
	`, req.TaskID, req.DependsOnTaskID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		writeError(w, http.StatusConflict, "dependency already exists")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create dependency")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to commit dependency")
		return
	}
	writeJSON(w, http.StatusCreated, models.TaskDependency{TaskID: req.TaskID, DependsOnTaskID: req.DependsOnTaskID})
}

// Delete godoc
// @Summary      Remove a dependency
// @Tags         dependencies
// @Produce      json
// @Param        dagId  path  string  true  "DAG ID"
// @Param        taskId  path  string  true  "Task that depends on the other"
// @Param        dependsOnId  path  string  true  "Task it depends on"
// @Success      204  "No Content"
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{dagId}/dependencies/{taskId}/{dependsOnId} [delete]
func (h *DependencyHandler) Delete(w http.ResponseWriter, r *http.Request) {
	dagID, err1 := uuid.Parse(r.PathValue("dagId"))
	taskID, err2 := uuid.Parse(r.PathValue("taskId"))
	dependsOnID, err3 := uuid.Parse(r.PathValue("dependsOnId"))
	if err1 != nil || err2 != nil || err3 != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}

	tag, err := h.DB.Exec(r.Context(), `
		DELETE FROM task_dependencies td
		USING tasks t
		WHERE td.task_id = t.id AND t.dag_id = $1 AND td.task_id = $2 AND td.depends_on_task_id = $3
	`, dagID, taskID, dependsOnID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete dependency")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "dependency not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
