package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

type Store interface {
	GetQueues(ctx context.Context) ([]Queue, error)
	CreateOrUpdateQueue(ctx context.Context, queue *Queue) error
	GetQueue(ctx context.Context, name string) (*Queue, error)
	CreateTask(ctx context.Context, task *Task) error
	UpdateTask(ctx context.Context, task *Task) error
	GetTask(ctx context.Context, id string) (*Task, error)
	GetTasks(ctx context.Context, filter TaskFilter) ([]Task, error)
	GetTaskStats(ctx context.Context, filter TaskFilter) (map[string]int, error)
	GetNextPendingTask(ctx context.Context, queueName, clientID string) (*Task, error)
	DeleteTask(ctx context.Context, id string) error
	MarkExpiredTasks(ctx context.Context) error
}

type store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) Store {
	return &store{db: db}
}

func (s *store) CreateOrUpdateQueue(ctx context.Context, queue *Queue) error {
	query := `
		INSERT INTO queues (name, task_timeout, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (name) 
		DO UPDATE SET 
			task_timeout = $2,
			updated_at = NOW()
		RETURNING created_at, updated_at`

	return s.db.QueryRowContext(ctx, query, queue.Name, queue.TaskTimeoutSeconds()).
		Scan(&queue.CreatedAt, &queue.UpdatedAt)
}

// func (s *store) GetQueue(ctx context.Context, name string) (*Queue, error) {
// 	queue := &Queue{}
// 	query := `
// 		SELECT name, task_timeout, created_at, updated_at
// 		FROM queues
// 		WHERE name = $1`

// 	err := s.db.QueryRowContext(ctx, query, name).
// 		Scan(&queue.Name, &queue.TaskTimeout, &queue.CreatedAt, &queue.UpdatedAt)
// 	if err == sql.ErrNoRows {
// 		return nil, nil
// 	}
// 	if err != nil {
// 		return nil, err
// 	}
// 	return queue, nil
// }

func (s *store) GetQueue(ctx context.Context, name string) (*Queue, error) {
	var queue Queue
	err := s.db.QueryRowContext(ctx, `
        SELECT 
            name, 
            task_timeout, 
            created_at, 
            updated_at
        FROM queues
        WHERE name = $1`,
		name,
	).Scan(
		&queue.Name,
		&queue.TaskTimeout,
		&queue.CreatedAt,
		&queue.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error getting queue: %w", err)
	}

	// Convertir segundos a Duration
	queue.TaskTimeout = queue.TaskTimeout * time.Second

	return &queue, nil
}

func (s *store) GetQueues(ctx context.Context) ([]Queue, error) {
	rows, err := s.db.QueryContext(ctx, `
        SELECT 
            name, 
            task_timeout, 
            created_at, 
            updated_at
        FROM queues
        ORDER BY name ASC`)
	if err != nil {
		return nil, fmt.Errorf("error querying queues: %w", err)
	}
	defer rows.Close()

	var queues []Queue
	for rows.Next() {
		var queue Queue
		var timeoutSeconds int64
		err := rows.Scan(
			&queue.Name,
			&timeoutSeconds,
			&queue.CreatedAt,
			&queue.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("error scanning queue: %w", err)
		}
		queue.TaskTimeout = time.Duration(timeoutSeconds) * time.Second
		queues = append(queues, queue)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating queues: %w", err)
	}

	return queues, nil
}

func (s *store) CreateTask(ctx context.Context, task *Task) error {
	query := `
		INSERT INTO tasks (id, queue_name, status, data, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		RETURNING created_at, updated_at`

	return s.db.QueryRowContext(ctx, query, task.ID, task.QueueName, task.Status, task.Data).
		Scan(&task.CreatedAt, &task.UpdatedAt)
}

func (s *store) UpdateTask(ctx context.Context, task *Task) error {
	query := `
		UPDATE tasks 
		SET status = $1,
			data = $2,
			assigned_to = $3,
			started_at = $4,
			completed_at = $5,
			updated_at = NOW()
		WHERE id = $6
		RETURNING created_at, updated_at`

	return s.db.QueryRowContext(ctx, query,
		task.Status, task.Data, task.AssignedTo, task.StartedAt, task.CompletedAt, task.ID).
		Scan(&task.CreatedAt, &task.UpdatedAt)
}

func (s *store) GetTask(ctx context.Context, id string) (*Task, error) {
	task := &Task{}
	query := `
		SELECT id, queue_name, status, data, assigned_to, created_at, updated_at, started_at, completed_at
		FROM tasks
		WHERE id = $1`

	err := s.db.QueryRowContext(ctx, query, id).
		Scan(&task.ID, &task.QueueName, &task.Status, &task.Data, &task.AssignedTo,
			&task.CreatedAt, &task.UpdatedAt, &task.StartedAt, &task.CompletedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return task, nil
}

func (s *store) GetTasks(ctx context.Context, filter TaskFilter) ([]Task, error) {
	var conditions []string
	var args []interface{}
	argCount := 1

	if filter.QueueName != "" {
		conditions = append(conditions, fmt.Sprintf("queue_name = $%d", argCount))
		args = append(args, filter.QueueName)
		argCount++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argCount))
		args = append(args, filter.Status)
		argCount++
	}

	if !filter.FromDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argCount))
		args = append(args, filter.FromDate)
		argCount++
	}

	if !filter.ToDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argCount))
		args = append(args, filter.ToDate)
		argCount++
	}

	query := "SELECT id, queue_name, status, data, assigned_to, created_at, updated_at, started_at, completed_at FROM tasks"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Sorting
	if filter.SortBy != "" {
		query += fmt.Sprintf(" ORDER BY %s", pq.QuoteIdentifier(filter.SortBy))
	} else {
		query += " ORDER BY created_at DESC"
	}

	// Pagination
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var task Task
		err := rows.Scan(
			&task.ID, &task.QueueName, &task.Status, &task.Data, &task.AssignedTo,
			&task.CreatedAt, &task.UpdatedAt, &task.StartedAt, &task.CompletedAt,
		)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

func (s *store) GetTaskStats(ctx context.Context, filter TaskFilter) (map[string]int, error) {
	var conditions []string
	var args []interface{}
	argCount := 1

	if filter.QueueName != "" {
		conditions = append(conditions, fmt.Sprintf("queue_name = $%d", argCount))
		args = append(args, filter.QueueName)
		argCount++
	}

	if filter.Status != "" {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argCount))
		args = append(args, filter.Status)
		argCount++
	}

	if !filter.FromDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argCount))
		args = append(args, filter.FromDate)
		argCount++
	}

	if !filter.ToDate.IsZero() {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argCount))
		args = append(args, filter.ToDate)
		argCount++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
        SELECT 
            COUNT(*) as total,
            COUNT(CASE WHEN status = 'pending' THEN 1 END) as pending,
            COUNT(CASE WHEN status = 'running' THEN 1 END) as running,
            COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed,
            COUNT(CASE WHEN status = 'failed' THEN 1 END) as failed,
            COUNT(CASE WHEN status = 'deleted' THEN 1 END) as deleted
        FROM tasks
        %s`, whereClause)

	var stats struct {
		Total     int
		Pending   int
		Running   int
		Completed int
		Failed    int
		Deleted   int
	}

	err := s.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.Total,
		&stats.Pending,
		&stats.Running,
		&stats.Completed,
		&stats.Failed,
		&stats.Deleted,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting statistics: %w", err)
	}

	return map[string]int{
		"all":       stats.Total,
		"pending":   stats.Pending,
		"running":   stats.Running,
		"completed": stats.Completed,
		"failed":    stats.Failed,
		"deleted":   stats.Deleted,
	}, nil
}

// func (s *store) GetNextPendingTask(ctx context.Context, queueName, clientID string) (*Task, error) {
// 	tx, err := s.db.BeginTx(ctx, nil)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer tx.Rollback()

// 	task := &Task{}
// 	query := `
// 		SELECT id, queue_name, status, data, created_at, updated_at
// 		FROM tasks
// 		WHERE queue_name = $1 AND status = $2
// 		ORDER BY created_at ASC
// 		LIMIT 1
// 		FOR UPDATE SKIP LOCKED`

// 	err = tx.QueryRowContext(ctx, query, queueName, TaskStatusPending).
// 		Scan(&task.ID, &task.QueueName, &task.Status, &task.Data,
// 			&task.CreatedAt, &task.UpdatedAt)
// 	if err == sql.ErrNoRows {
// 		return nil, nil
// 	}
// 	if err != nil {
// 		return nil, err
// 	}

// 	now := time.Now()
// 	task.Status = TaskStatusRunning
// 	task.AssignedTo = &clientID
// 	task.StartedAt = &now
// 	task.UpdatedAt = now

// 	updateQuery := `
// 		UPDATE tasks
// 		SET status = $1, assigned_to = $2, started_at = $3, updated_at = $3
// 		WHERE id = $4`

// 	_, err = tx.ExecContext(ctx, updateQuery,
// 		task.Status, task.AssignedTo, task.StartedAt, task.ID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	if err = tx.Commit(); err != nil {
// 		return nil, err
// 	}

// 	return task, nil
// }

func (s *store) GetNextPendingTask(ctx context.Context, queueName, clientID string) (*Task, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("error starting transaction: %w", err)
	}
	defer tx.Rollback()

	// get queue timeout
	var queueTimeout int
	err = tx.QueryRowContext(ctx, `
        SELECT task_timeout 
        FROM queues 
        WHERE name = $1`,
		queueName,
	).Scan(&queueTimeout)
	if err != nil {
		return nil, fmt.Errorf("error getting queue timeout: %w", err)
	}

	// get next pending task
	task := &Task{}
	err = tx.QueryRowContext(ctx, `
        SELECT id, queue_name, status, data, created_at, updated_at
        FROM tasks
        WHERE queue_name = $1 AND status = $2 AND assigned_to IS NULL
        ORDER BY created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED`,
		queueName, TaskStatusPending,
	).Scan(&task.ID, &task.QueueName, &task.Status, &task.Data,
		&task.CreatedAt, &task.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("error getting next task: %w", err)
	}

	now := time.Now()
	task.Status = TaskStatusRunning
	task.AssignedTo = &clientID
	task.StartedAt = &now
	task.UpdatedAt = now

	// update task status with assigned client
	_, err = tx.ExecContext(ctx, `
        UPDATE tasks 
        SET status = $1, 
            assigned_to = $2, 
            started_at = $3, 
            updated_at = $3
        WHERE id = $4`,
		task.Status, task.AssignedTo, task.StartedAt, task.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("error updating task status: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("error committing transaction: %w", err)
	}

	return task, nil
}

func (s *store) DeleteTask(ctx context.Context, id string) error {
	query := `
		UPDATE tasks 
		SET status = $1, updated_at = NOW()
		WHERE id = $2`

	result, err := s.db.ExecContext(ctx, query, TaskStatusDeleted, id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return fmt.Errorf("task not found")
	}

	return nil
}

// mark expired tasks as failed with error message when the task timeout is exceeded
func (s *store) MarkExpiredTasks(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
        UPDATE tasks t
        SET 
            status = 'failed',
            updated_at = NOW(),
            data = jsonb_set(
                CASE 
                    WHEN jsonb_typeof(data) = 'object' THEN data 
                    ELSE '{}'::jsonb 
                END, 
                '{error}', 
                '"Task timeout exceeded"'
            )
        FROM queues q
        WHERE 
            t.queue_name = q.name
            AND t.status = 'running'
            AND t.started_at + (q.task_timeout || ' seconds')::interval < NOW()`)

	if err != nil {
		return fmt.Errorf("error marking expired tasks: %w", err)
	}
	return nil
}
