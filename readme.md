# Job Queue Service

A robust, distributed job queue service built with Go, featuring a RESTful API, persistent storage with PostgreSQL, and a (near) real-time web dashboard.

## Features

- RESTful API for job queue management
- Persistent storage with PostgreSQL
- Configurable task timeouts per queue
- Parallel task processing
- Real-time web dashboard
- Docker support
- Go client library included

## Quick Start with Docker

```bash
# Clone the repository
git clone https://github.com/fernandezvara/jobqueue
cd jobqueue

# Start the service
docker-compose up -d
```


> The service will be available at:
>- API: http://localhost:8080/api/v1
>- Dashboard: http://localhost:8080/dashboard

## API Documentation

### Queues

#### Create/Update Queue
```http
PUT /api/v1/queues/{queue-name}
Content-Type: application/json

{
    "task_timeout": 3600
}
```
Response:
```json
{
    "name": "my-queue",
    "task_timeout": 3600,
    "created_at": "2024-01-01T12:00:00Z",
    "updated_at": "2024-01-01T12:00:00Z"
}
```

#### List Queues
```http
GET /api/v1/queues
```
Response:
```json
[
    {
        "name": "my-queue",
        "task_timeout": 3600,
        "created_at": "2024-01-01T12:00:00Z",
        "updated_at": "2024-01-01T12:00:00Z"
    }
]
```

#### Get Queue
```http
GET /api/v1/queues/{queue-name}
```

### Tasks

#### Create Task
```http
POST /api/v1/tasks
Content-Type: application/json

{
    "queue_name": "my-queue",
    "data": {
        "key": "value"
    }
}
```
Response:
```json
{
    "id": "ck8v0g90000001la7w1fah3jk",
    "queue_name": "my-queue",
    "status": "pending",
    "data": {
        "key": "value"
    },
    "created_at": "2024-01-01T12:00:00Z",
    "updated_at": "2024-01-01T12:00:00Z"
}
```

#### Get Next Task
```http
GET /api/v1/tasks/next?queue={queue-name}
X-Client-ID: worker-1
```

#### List Tasks
```http
GET /api/v1/tasks?queue={name}&status={status}&from={epoch}&to={epoch}&sort_by={field}&offset={offset}&limit={limit}
```

Optional query parameter `summary=true` returns statistics instead of task list:
```json
{
    "all": 100,
    "pending": 10,
    "running": 5,
    "completed": 80,
    "failed": 5,
    "deleted": 0
}
```

#### Update Task
```http
PUT /api/v1/tasks/{task-id}
Content-Type: application/json

{
    "status": "completed",
    "data": {
        "result": "success"
    }
}
```

#### Delete Task
```http
DELETE /api/v1/tasks/{task-id}
```

## Client Library Usage

There is a basic client example at `cmd/clientexample/main.go`

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/fernandezvara/jobqueue/pkg/jobqueue"
)

func main() {
    // Create client
    client := jobqueue.NewClient("http://localhost:8080",
        jobqueue.WithClientID("worker-1"), // skip it if you want to identify the client as 'hostname-process id'
        jobqueue.WithTimeout(30*time.Second),
    )

    // Create a queue
    queue, err := client.CreateOrUpdateQueue(context.Background(), "my-queue", 1*time.Hour)
    if err != nil {
        log.Fatal(err)
    }

    // Create a task
    task, err := client.CreateTask(context.Background(), "my-queue", map[string]interface{}{
        "key": "value",
    })
    if err != nil {
        log.Fatal(err)
    }

    // Process tasks with parallel workers
    config := jobqueue.ProcessTasksConfig{
        QueueName:    "my-queue",
        WorkerCount:  5,           // 5 parallel workers
        WorkerBuffer: 10,          // Buffer size for tasks
    }

    err = client.ProcessTasks(context.Background(), config, func(ctx context.Context, task *jobqueue.Task) error {
        // Process the task
        log.Printf("Processing task: %s", task.ID)
        return nil
    })
}
```

### Task Processing with Timeout

The client respects queue-defined timeouts:

```go
config := jobqueue.ProcessTasksConfig{
    QueueName:     "my-queue",
    RetryInterval: 5 * time.Second,
    WorkerCount:   3,
    StopOnError:   true,
}

err = client.ProcessTasks(ctx, config, func(ctx context.Context, task *jobqueue.Task) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    default:
        // Your processing logic here
        return nil
    }
})
```

## Development Setup

### Requirements

- Go 1.21 or higher
- PostgreSQL 12 or higher
- Docker and Docker Compose (optional)

### Local Development

1. Clone the repository
```bash
git clone https://github.com/fernandezvara/jobqueue
cd jobqueue
```

2. Install dependencies
```bash
go mod download
```

3. Start PostgreSQL (if using Docker)
```bash
docker-compose up -d postgres
```

4. Run the service
```bash
go run cmd/jobqueue/main.go
```

You can run the service and the database using `docker-compose up`. It will build the local image for the service and bring up the local environment.

### Environment Variables

- `DATABASE_URL`: PostgreSQL connection string (default: "postgresql://jobqueue:jobqueue@localhost:5432/jobqueue?sslmode=disable")
- `PORT`: API server port (default: "8080")

## License

MIT License - see LICENSE file for details.