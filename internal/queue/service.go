package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/fernandezvara/jobqueues/internal/storage"
	"github.com/rs/xid"
)

type Service interface {
	GetQueue(ctx context.Context, name string) (*storage.Queue, error)
	GetQueues(ctx context.Context) ([]storage.Queue, error)
	CreateOrUpdateQueue(ctx context.Context, queue *storage.Queue) error
	CreateTask(ctx context.Context, task *storage.Task) error
	UpdateTask(ctx context.Context, task *storage.Task) error
	GetTask(ctx context.Context, id string) (*storage.Task, error)
	GetTasks(ctx context.Context, filter storage.TaskFilter) ([]storage.Task, error)
	GetTaskStats(ctx context.Context, filter storage.TaskFilter) (map[string]int, error)
	GetNextTask(ctx context.Context, queueName, clientID string) (*storage.Task, error)
	DeleteTask(ctx context.Context, id string) error
	Shutdown() error
}

type service struct {
	store         storage.Store
	timeoutWorker *TimeoutWorker
}

func NewService(store storage.Store) Service {
	s := &service{
		store:         store,
		timeoutWorker: NewTimeoutWorker(store, 30*time.Second),
	}
	s.timeoutWorker.Start()
	return s
}

func (s *service) GetQueue(ctx context.Context, name string) (*storage.Queue, error) {
	if name == "" {
		return nil, fmt.Errorf("queue name is required")
	}
	return s.store.GetQueue(ctx, name)
}

func (s *service) GetQueues(ctx context.Context) ([]storage.Queue, error) {
	return s.store.GetQueues(ctx)
}

func (s *service) CreateOrUpdateQueue(ctx context.Context, queue *storage.Queue) error {
	if queue.Name == "" {
		return fmt.Errorf("queue name is required")
	}
	if queue.TaskTimeout <= 0 {
		return fmt.Errorf("task timeout must be positive")
	}
	return s.store.CreateOrUpdateQueue(ctx, queue)
}

func (s *service) CreateTask(ctx context.Context, task *storage.Task) error {
	if task.QueueName == "" {
		return fmt.Errorf("queue name is required")
	}

	// Verify that the queue exists
	queue, err := s.store.GetQueue(ctx, task.QueueName)
	if err != nil {
		return fmt.Errorf("error checking queue: %w", err)
	}
	if queue == nil {
		return fmt.Errorf("queue %s does not exist", task.QueueName)
	}

	// Generate unique ID
	task.ID = xid.New().String()
	task.Status = storage.TaskStatusPending

	return s.store.CreateTask(ctx, task)
}

func (s *service) UpdateTask(ctx context.Context, task *storage.Task) error {
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}

	// Verify that the task exists
	existingTask, err := s.store.GetTask(ctx, task.ID)
	if err != nil {
		return fmt.Errorf("error checking task: %w", err)
	}
	if existingTask == nil {
		return fmt.Errorf("task %s does not exist", task.ID)
	}

	// Validate status transitions
	if !isValidStatusTransition(existingTask.Status, task.Status) {
		return fmt.Errorf("invalid status transition from %s to %s", existingTask.Status, task.Status)
	}

	return s.store.UpdateTask(ctx, task)
}

func (s *service) GetTask(ctx context.Context, id string) (*storage.Task, error) {
	if id == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	return s.store.GetTask(ctx, id)
}

func (s *service) GetTasks(ctx context.Context, filter storage.TaskFilter) ([]storage.Task, error) {
	if filter.Limit <= 0 {
		filter.Limit = 10 // default value
	}
	if filter.Limit > 100 {
		filter.Limit = 100 // maximum limit
	}
	return s.store.GetTasks(ctx, filter)
}

func (s *service) GetTaskStats(ctx context.Context, filter storage.TaskFilter) (map[string]int, error) {
	return s.store.GetTaskStats(ctx, filter)
}

func (s *service) GetNextTask(ctx context.Context, queueName, clientID string) (*storage.Task, error) {
	if queueName == "" {
		return nil, fmt.Errorf("queue name is required")
	}
	if clientID == "" {
		return nil, fmt.Errorf("client ID is required")
	}

	// Verify that the queue exists
	queue, err := s.store.GetQueue(ctx, queueName)
	if err != nil {
		return nil, fmt.Errorf("error checking queue: %w", err)
	}
	if queue == nil {
		return nil, fmt.Errorf("queue %s does not exist", queueName)
	}

	return s.store.GetNextPendingTask(ctx, queueName, clientID)
}

func (s *service) DeleteTask(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("task ID is required")
	}
	return s.store.DeleteTask(ctx, id)
}

func (s *service) Shutdown() error {
	s.timeoutWorker.Stop()
	return nil
}

func isValidStatusTransition(from, to string) bool {
	validTransitions := map[string][]string{
		storage.TaskStatusPending: {
			storage.TaskStatusRunning,
			storage.TaskStatusDeleted,
		},
		storage.TaskStatusRunning: {
			storage.TaskStatusCompleted,
			storage.TaskStatusFailed,
			storage.TaskStatusDeleted,
		},
		storage.TaskStatusCompleted: {
			storage.TaskStatusDeleted,
		},
		storage.TaskStatusFailed: {
			storage.TaskStatusPending,
			storage.TaskStatusDeleted,
		},
		storage.TaskStatusDeleted: {},
	}

	allowedTransitions, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, allowed := range allowedTransitions {
		if to == allowed {
			return true
		}
	}
	return false
}
