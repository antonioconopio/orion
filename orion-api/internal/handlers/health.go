package handlers

import "net/http"

// Health handles GET /health — a cheap endpoint for checking the server
// is up. Load balancers, Docker healthchecks, and monitoring tools all
// expect something like this on every real service.
//
// Health godoc
// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}