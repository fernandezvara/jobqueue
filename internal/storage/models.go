package storage

import (
	"encoding/json"
	"time"
)

type Queue struct {
	Name        string        `json:"name"`
	TaskTimeout time.Duration `json:"task_timeout"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// TaskTimeoutSeconds is a helper method to convert the task timeout to seconds for database storage
func (q Queue) TaskTimeoutSeconds() int64 {
	return int64(q.TaskTimeout.Seconds())
}

type Task struct {
	ID          string          `json:"id"`
	QueueName   string          `json:"queue_name"`
	Status      string          `json:"status"`
	Data        json.RawMessage `json:"data"`
	AssignedTo  *string         `json:"assigned_to"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	StartedAt   *time.Time      `json:"started_at"`
	CompletedAt *time.Time      `json:"completed_at"`
}

type TaskFilter struct {
	QueueName string
	Status    string
	FromDate  time.Time
	ToDate    time.Time
	SortBy    string
	Offset    int
	Limit     int
}

const (
	TaskStatusPending   = "pending"
	TaskStatusRunning   = "running"
	TaskStatusCompleted = "completed"
	TaskStatusFailed    = "failed"
	TaskStatusDeleted   = "deleted"
)
