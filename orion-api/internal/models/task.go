package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

//   id, dag_id, name, task_type, config, created_at
//
// Hint: config is JSONB in Postgres — use json.RawMessage as its Go type.
// That lets pgx hand you the raw bytes without needing to know the shape
// of every task type's config ahead of time.
type Task struct {
	ID         uuid.UUID     `json:"id"`
	DAGID	   uuid.UUID     `json:"dag_id"`
	Name	   string        `json:"name"`
	TaskType   string        `json:"task_type"`
	Config     json.RawMessage `json:"config" swaggertype:"object"`
	CreatedAt  time.Time      `json:"created_at"`
}

// create a task? (Compare to CreateDAGRequest in dag.go — what's the
// equivalent set of fields here, minus anything the DB generates?)
type CreateTaskRequest struct {
	Name      string          `json:"name"`
	TaskType  string          `json:"task_type"`
	Config    json.RawMessage `json:"config" swaggertype:"object"`
}
