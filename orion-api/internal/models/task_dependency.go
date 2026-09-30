package models

import (
	"github.com/google/uuid"
)

type TaskDependency struct {
	TaskID	   		uuid.UUID `json:"task_id"`
	DependsOnTaskID uuid.UUID `json:"depends_on_task_id"`
}

type CreateTaskDependencyRequest struct {
	TaskID	   		uuid.UUID `json:"task_id"`
	DependsOnTaskID uuid.UUID `json:"depends_on_task_id"`
}
