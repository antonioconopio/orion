package models

// Stats is what the dashboard shows.
type Stats struct {
	ActiveRuns     int      `json:"active_runs"`      // queued or running
	SuccessRate24h *float64 `json:"success_rate_24h"` // 0..1; null when nothing finished in 24h
	Failed24h      int      `json:"failed_24h"`
	Throughput24h  []int    `json:"throughput_24h"` // runs finished per hour, oldest first, 24 entries
}
