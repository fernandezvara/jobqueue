package jobqueue

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client represents the Job Queue API client
type Client struct {
	baseURL    string
	httpClient *http.Client
	clientID   string
}

// ClientOption is a function that configures the client
type ClientOption func(*Client)

// WithTimeout configures the HTTP client timeout
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.httpClient.Timeout = timeout
	}
}

// WithClientID configures the client ID for processing tasks
func WithClientID(clientID string) ClientOption {
	return func(c *Client) {
		c.clientID = clientID
	}
}

// WithHTTPClient allows using a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// ProcessTasksConfig contains the configuration for task processing
type ProcessTasksConfig struct {
	QueueName     string
	RetryInterval time.Duration // Interval between attempts when there are no tasks
	StopOnError   bool          // Whether to stop when an error is encountered
	PreserveError bool          // Whether to preserve the original error in the task data
	WorkerCount   int           // Number of workers to process tasks
	WorkerBuffer  int           // Buffer size for the worker channel
}

// DefaultProcessTasksConfig returns a default configuration
func DefaultProcessTasksConfig(queueName string) ProcessTasksConfig {
	return ProcessTasksConfig{
		QueueName:     queueName,
		RetryInterval: 5 * time.Second,
		StopOnError:   false,
		PreserveError: true,
		WorkerCount:   1,  // Default to 1 worker
		WorkerBuffer:  10, // Default buffer size
	}
}

// Queue represents a task queue
type Queue struct {
	Name        string        `json:"name"`
	TaskTimeout time.Duration `json:"task_timeout"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Helper methods for Queue
func (q Queue) TimeoutSeconds() int64 {
	return int64(q.TaskTimeout.Seconds())
}

func (q Queue) String() string {
	return fmt.Sprintf("Queue{Name: %s, Timeout: %v}", q.Name, q.TaskTimeout)
}

// Task represents a task in the queue
type Task struct {
	ID          string          `json:"id"`
	QueueName   string          `json:"queue_name"`
	Status      string          `json:"status"`
	Data        json.RawMessage `json:"data"`
	AssignedTo  *string         `json:"assigned_to,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	StartedAt   *time.Time      `json:"started_at,omitempty"`
	CompletedAt *time.Time      `json:"completed_at,omitempty"`
}

// HealthStatus represents the health status of the service
type HealthStatus struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
}

// APIError is a custom error for the API
type APIError struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error: %d - %s", e.StatusCode, e.Message)
}

// taskResult represents the result of a task processing
type taskResult struct {
	task *Task
	err  error
	data json.RawMessage
}
