package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
)

// DAGHandler bundles everything the DAG endpoints need. Right now that's
// just the DB pool, but this is where you'd add a logger, config, etc.
// later without changing every function signature.
type DAGHandler struct {
	DB *pgxpool.Pool
}

func NewDAGHandler(db *pgxpool.Pool) *DAGHandler {
	return &DAGHandler{DB: db}
}

// writeJSON is a tiny helper so every handler doesn't repeat the same
// three lines of header-setting and encoding boilerplate.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// List handles GET /dags — returns every DAG in the system.
//
// List godoc
// @Summary      List all DAGs
// @Tags         dags
// @Produce      json
// @Success      200  {array}  models.DAG
// @Failure      500  {object}  map[string]string
// @Router       /dags [get]
func (h *DAGHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(r.Context(), `
		SELECT id, name, description, schedule, owner, is_active, created_at, updated_at
		FROM dags
		ORDER BY created_at DESC
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query dags")
		return
	}
	defer rows.Close()

	dags := []models.DAG{}
	for rows.Next() {
		var d models.DAG
		if err := rows.Scan(&d.ID, &d.Name, &d.Description, &d.Schedule, &d.Owner, &d.IsActive, &d.CreatedAt, &d.UpdatedAt); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to read dag row")
			return
		}
		dags = append(dags, d)
	}

	writeJSON(w, http.StatusOK, dags)
}

// Get handles GET /dags/{id} — returns one DAG by its UUID.
//
// Get godoc
// @Summary      Get a DAG
// @Tags         dags
// @Produce      json
// @Param        id  path  string  true  "DAG ID"
// @Success      200  {object}  models.DAG
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{id} [get]
func (h *DAGHandler) Get(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	var d models.DAG
	err = h.DB.QueryRow(r.Context(), `
		SELECT id, name, description, schedule, owner, is_active, created_at, updated_at
		FROM dags
		WHERE id = $1
	`, id).Scan(&d.ID, &d.Name, &d.Description, &d.Schedule, &d.Owner, &d.IsActive, &d.CreatedAt, &d.UpdatedAt)

	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "dag not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query dag")
		return
	}

	writeJSON(w, http.StatusOK, d)
}

// Create handles POST /dags — registers a new DAG.
//
// Create godoc
// @Summary      Create a DAG
// @Tags         dags
// @Accept       json
// @Produce      json
// @Param        dag  body  models.CreateDAGRequest  true  "DAG to create"
// @Success      201  {object}  models.DAG
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags [post]
func (h *DAGHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req models.CreateDAGRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	var d models.DAG
	err := h.DB.QueryRow(r.Context(), `
		INSERT INTO dags (name, description, schedule, owner)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, schedule, owner, is_active, created_at, updated_at
	`, req.Name, req.Description, req.Schedule, req.Owner).Scan(
		&d.ID, &d.Name, &d.Description, &d.Schedule, &d.Owner, &d.IsActive, &d.CreatedAt, &d.UpdatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create dag")
		return
	}

	writeJSON(w, http.StatusCreated, d)
}

// Delete godoc
// @Summary      Delete a DAG
// @Tags         dags
// @Produce      json
// @Param        id  path  string  true  "DAG ID"
// @Success      204  "No Content"
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{id} [delete]
func (h *DAGHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	cmdTag, err := h.DB.Exec(r.Context(), `
		DELETE FROM dags WHERE id = $1
	`, id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete dag")
		return
	}
	if cmdTag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "dag not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Update godoc
// @Summary      Update a DAG
// @Tags         dags
// @Accept       json
// @Produce      json
// @Param        id  path  string  true  "DAG ID"
// @Param        dag  body  models.UpdateDAGRequest  true  "Fields to update"
// @Success      200  {object}  models.DAG
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Router       /dags/{id} [put]
func (h *DAGHandler) Update(w http.ResponseWriter, r *http.Request) {
	idParam := r.PathValue("id")
	id, err := uuid.Parse(idParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid dag id")
		return
	}

	var req models.UpdateDAGRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	var d models.DAG
	err = h.DB.QueryRow(r.Context(), `
		UPDATE dags
		SET name = COALESCE($1, name),
		    description = COALESCE($2, description),
		    schedule = COALESCE($3, schedule),
		    owner = COALESCE($4, owner),
		    is_active = COALESCE($5, is_active),
		    updated_at = NOW()
		WHERE id = $6
		RETURNING id, name, description, schedule, owner, is_active, created_at, updated_at
	`, req.Name, req.Description, req.Schedule, req.Owner, req.IsActive, id).Scan(
		&d.ID, &d.Name, &d.Description, &d.Schedule, &d.Owner, &d.IsActive, &d.CreatedAt, &d.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		writeError(w, http.StatusNotFound, "dag not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update dag")
		return
	}

	writeJSON(w, http.StatusOK, d)
}
