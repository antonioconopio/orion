package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type TaskInstance struct {
	ID         uuid.UUID       `json:"id"`
	RunID      uuid.UUID       `json:"run_id"`
	TaskID     uuid.UUID       `json:"task_id"`
	Status     string          `json:"status"`
	Attempt    int             `json:"attempt"`
	WorkerID   *string         `json:"worker_id"`
	StartedAt  *time.Time      `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at"`
	Logs       *string         `json:"logs"`
	Output     json.RawMessage `json:"output" swaggertype:"object"`
}

type CreateTaskInstanceRequest struct {
	RunID   	uuid.UUID     	`json:"run_id"`
	TaskID  	uuid.UUID     	`json:"task_id"`
	Status  	string        	`json:"status"`
	Attempt 	int           	`json:"attempt"`
}