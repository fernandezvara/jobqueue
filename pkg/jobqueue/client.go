package jobqueue

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// NewClient creates a new instance of the client
func NewClient(baseURL string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	if c.clientID == "" {
		hostname, _ := os.Hostname()
		c.clientID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}

	return c
}

// ClientID returns the client ID configured
func (c *Client) ClientID() string {
	return c.clientID
}

// Health checks the health status of the service
func (c *Client) Health(ctx context.Context) (*HealthStatus, error) {
	var status HealthStatus
	err := c.doRequest(ctx, http.MethodGet, "/health", nil, &status)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

// CreateOrUpdateQueue creates or updates a queue
func (c *Client) CreateOrUpdateQueue(ctx context.Context, name string, timeout time.Duration) (*Queue, error) {
	queue := Queue{
		Name:        name,
		TaskTimeout: timeout,
	}

	var result Queue
	err := c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v1/queues/%s", name), queue, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetQueue gets the details of a queue
func (c *Client) GetQueue(ctx context.Context, name string) (*Queue, error) {
	if name == "" {
		return nil, fmt.Errorf("queue name is required")
	}

	var queue Queue
	err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/api/v1/queues/%s", name), nil, &queue)
	if err != nil {
		return nil, err
	}
	return &queue, nil
}

// GetQueues obtiene la lista de todas las colas
func (c *Client) GetQueues(ctx context.Context) ([]Queue, error) {
	var queues []Queue
	err := c.doRequest(ctx, http.MethodGet, "/api/v1/queues", nil, &queues)
	if err != nil {
		return nil, err
	}
	return queues, nil
}

// CreateTask creates a new task
func (c *Client) CreateTask(ctx context.Context, queueName string, data interface{}) (*Task, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling data: %w", err)
	}

	task := struct {
		QueueName string          `json:"queue_name"`
		Data      json.RawMessage `json:"data"`
	}{
		QueueName: queueName,
		Data:      jsonData,
	}

	var result Task
	err = c.doRequest(ctx, http.MethodPost, "/api/v1/tasks", task, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTasks retrieves the list of tasks based on filters
func (c *Client) GetTasks(ctx context.Context, filter TaskFilter) ([]Task, error) {
	queryParams := filter.toQueryParams()

	var tasks []Task
	err := c.doRequest(ctx, http.MethodGet, "/api/v1/tasks?"+queryParams.Encode(), nil, &tasks)
	if err != nil {
		return nil, err
	}

	return tasks, nil
}

func (c *Client) GetTaskStats(ctx context.Context, filter TaskFilter) (map[string]int, error) {
	queryParams := filter.toQueryParams()
	queryParams.Set("summary", "true")

	var stats map[string]int
	err := c.doRequest(ctx, http.MethodGet, "/api/v1/tasks?"+queryParams.Encode(), nil, &stats)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// UpdateTask updates an existing task
func (c *Client) UpdateTask(ctx context.Context, id string, status string, data interface{}) (*Task, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("error marshaling data: %w", err)
	}

	update := struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}{
		Status: status,
		Data:   jsonData,
	}

	var result Task
	err = c.doRequest(ctx, http.MethodPut, fmt.Sprintf("/api/v1/tasks/%s", id), update, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteTask deletes a task
func (c *Client) DeleteTask(ctx context.Context, id string) error {
	return c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/api/v1/tasks/%s", id), nil, nil)
}

// GetNextTask retrieves the next available task for processing
func (c *Client) GetNextTask(ctx context.Context, queueName string) (*Task, error) {
	// if c.clientID == "" {
	// 	return nil, fmt.Errorf("client ID is required for getting next task")
	// }

	query := url.Values{}
	query.Set("queue", queueName)

	var task Task
	err := c.doRequest(ctx, http.MethodGet, "/api/v1/tasks/next?"+query.Encode(), nil, &task)
	if err != nil {
		// If no tasks are available, return nil without error
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// DashboardURL returns the URL to access the dashboard
func (c *Client) DashboardURL() string {
	return fmt.Sprintf("%s/dashboard/", c.baseURL)
}

// doRequest performs the HTTP request and processes the response
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("error marshaling request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.clientID != "" {
		req.Header.Set("X-Client-ID", c.clientID)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	// Check if the response is successful
	if resp.StatusCode >= 400 {
		var apiError struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(respBody, &apiError); err == nil && apiError.Error != "" {
			return &APIError{
				StatusCode: resp.StatusCode,
				Message:    apiError.Error,
			}
		}
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    http.StatusText(resp.StatusCode),
		}
	}

	// If a result is expected, deserialize the response
	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("error unmarshaling response: %w", err)
		}
	}

	return nil
}
