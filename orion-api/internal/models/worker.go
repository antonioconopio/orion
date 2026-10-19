package models

import "time"

// Worker is a live worker process, read from its Redis heartbeat. A worker that stops
// heartbeating disappears from the list.
type Worker struct {
	ID           string    `json:"id"` // hostname-pid
	Host         string    `json:"host"`
	PID          int       `json:"pid"`
	StartedAt    time.Time `json:"started_at"`
	Status       string    `json:"status"`         // "idle" or "busy"
	CurrentTask  *string   `json:"current_task"`   // "dag / task" while busy
	CurrentRunID *string   `json:"current_run_id"` // for linking to the run
}
