package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"orion-api/internal/models"
)

type StatsHandler struct {
	DB *pgxpool.Pool
}

func NewStatsHandler(db *pgxpool.Pool) *StatsHandler {
	return &StatsHandler{DB: db}
}

// Get godoc
// @Summary      Dashboard numbers
// @Description  Active runs, 24h success rate and failures, and runs finished per hour for the last 24 hours (oldest first).
// @Tags         stats
// @Produce      json
// @Success      200  {object}  models.Stats
// @Failure      500  {object}  map[string]string
// @Router       /stats [get]
func (h *StatsHandler) Get(w http.ResponseWriter, r *http.Request) {
	var s models.Stats
	err := h.DB.QueryRow(r.Context(), `
		SELECT
			(SELECT count(*) FROM runs WHERE status IN ('queued', 'running')),
			(SELECT avg(CASE WHEN status = 'success' THEN 1.0 ELSE 0.0 END)::float8
			   FROM runs WHERE status IN ('success', 'failed') AND finished_at > NOW() - INTERVAL '24 hours'),
			(SELECT count(*) FROM runs WHERE status = 'failed' AND finished_at > NOW() - INTERVAL '24 hours')
	`).Scan(&s.ActiveRuns, &s.SuccessRate24h, &s.Failed24h)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query stats")
		return
	}

	rows, err := h.DB.Query(r.Context(), `
		SELECT count(r.id)
		FROM generate_series(date_trunc('hour', NOW()) - INTERVAL '23 hours', date_trunc('hour', NOW()), INTERVAL '1 hour') AS g(h)
		LEFT JOIN runs r ON r.status IN ('success', 'failed')
		                AND r.finished_at >= g.h AND r.finished_at < g.h + INTERVAL '1 hour'
		GROUP BY g.h ORDER BY g.h
	`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query throughput")
		return
	}
	defer rows.Close()

	s.Throughput24h = []int{}
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to scan throughput")
			return
		}
		s.Throughput24h = append(s.Throughput24h, n)
	}
	writeJSON(w, http.StatusOK, s)
}
