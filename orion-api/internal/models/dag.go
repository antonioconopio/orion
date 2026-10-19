package models

import (
	"time"

	"github.com/google/uuid"
)

// DAG mirrors one row of the "dags" table.
// Struct tags after each field tell the JSON encoder what key name to use
// when this struct is converted to/from JSON for the frontend.
type DAG struct {
	ID            uuid.UUID `json:"id"`
	Name          string    `json:"name"`
	Description   *string   `json:"description"` // pointer because it's nullable in the DB
	Schedule      *string   `json:"schedule"`
	Owner         *string   `json:"owner"`
	IsActive      bool      `json:"is_active"`
	PythonVersion *string   `json:"python_version"` // e.g. "3.11"; NULL = the worker's own interpreter
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// CreateDAGRequest is the shape of the JSON body we expect when someone
// POSTs a new DAG. It's a separate, smaller struct because the caller
// shouldn't be able to set ID, CreatedAt, etc. themselves — the database
// generates those.
type CreateDAGRequest struct {
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	Schedule      *string `json:"schedule"`
	Owner         *string `json:"owner"`
	PythonVersion *string `json:"python_version"`
}

type UpdateDAGRequest struct {
	Name          *string `json:"name"`
	Description   *string `json:"description"`
	Schedule      *string `json:"schedule"`
	Owner         *string `json:"owner"`
	IsActive      *bool   `json:"is_active"`
	PythonVersion *string `json:"python_version"`
}

// RunBrief is the slice of a run a DAG row needs.
type RunBrief struct {
	ID         uuid.UUID  `json:"id"`
	Number     int        `json:"number"`
	Status     string     `json:"status"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}

// DAGSummary is a DAG plus stats derived from its recent runs (list and get).
type DAGSummary struct {
	DAG
	LastRun            *RunBrief `json:"last_run"`
	SuccessRate        *float64  `json:"success_rate"`         // 0..1 over the last 20 finished runs
	AvgDurationSeconds *float64  `json:"avg_duration_seconds"` // over the same runs
	RecentDurations    []float64 `json:"recent_durations"`     // seconds, oldest to newest, for the sparkline
}
