package models

import (
	"time"

	"github.com/google/uuid"
)

type Run struct {
	ID        	uuid.UUID 	`json:"id"`
	DAGID    	uuid.UUID 	`json:"dag_id"`
	Status   	string    	`json:"status"`
	TriggeredBy string   	`json:"triggered_by"`
	StartedAt 	time.Time 	`json:"started_at"`
	FinishedAt 	time.Time 	`json:"finished_at"`
	CreatedAt 	time.Time 	`json:"created_at"`
}

type CreateRunRequest struct {
	DAGID    	uuid.UUID 	`json:"dag_id"`
	Status   	string    	`json:"status"`
	TriggeredBy string   	`json:"triggered_by"`
}