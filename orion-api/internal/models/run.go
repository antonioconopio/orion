package models

import (
	"time"

	"github.com/google/uuid"
)

type Run struct {
	ID          uuid.UUID  `json:"id"`
	DAGID       uuid.UUID  `json:"dag_id"`
	DAGName     string     `json:"dag_name"`
	Number      int        `json:"number"` // per-DAG sequence: the "#12" in the UI
	Status      string     `json:"status"`
	TriggeredBy string     `json:"triggered_by"`
	StartedAt   *time.Time `json:"started_at"`
	FinishedAt  *time.Time `json:"finished_at"`
	CreatedAt   time.Time  `json:"created_at"`
	TasksTotal  int        `json:"tasks_total"`
	TasksDone   int        `json:"tasks_done"` // success + failed + skipped
}

type CreateRunRequest struct {
	DAGID       uuid.UUID `json:"dag_id"`
	Status      string    `json:"status"`
	TriggeredBy string    `json:"triggered_by"`
}
