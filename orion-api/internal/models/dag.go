package models

import (
	"time"

	"github.com/google/uuid"
)

// DAG mirrors one row of the "dags" table.
// Struct tags after each field tell the JSON encoder what key name to use
// when this struct is converted to/from JSON for the frontend.
type DAG struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"` // pointer because it's nullable in the DB
	Schedule    *string   `json:"schedule"`
	Owner       *string   `json:"owner"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateDAGRequest is the shape of the JSON body we expect when someone
// POSTs a new DAG. It's a separate, smaller struct because the caller
// shouldn't be able to set ID, CreatedAt, etc. themselves — the database
// generates those.
type CreateDAGRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Schedule    *string `json:"schedule"`
	Owner       *string `json:"owner"`
}

type UpdateDAGRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Schedule    *string `json:"schedule"`
	Owner       *string `json:"owner"`
	IsActive    *bool   `json:"is_active"`
}
