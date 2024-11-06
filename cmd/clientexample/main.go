package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/fernandezvara/jobqueues/pkg/jobqueue"
)

func main() {

	// Queue name and timeout
	queueName := "example-queue3"
	queueTimeout := 10 * time.Second

	config := jobqueue.ProcessTasksConfig{
		QueueName:     queueName,
		RetryInterval: 10 * time.Second,
		StopOnError:   false,
		PreserveError: true,
		WorkerCount:   50,
		WorkerBuffer:  50,
	}

	// Create client
	client := jobqueue.NewClient(
		"http://localhost:8080",
		jobqueue.WithTimeout(15*time.Second),
		// jobqueue.WithClientID("example-client"),
	)

	// Create cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle termination signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		cancel()
	}()

	// Check service status
	status, err := client.Health(ctx)
	if err != nil {
		log.Fatalf("Error checking health: %v", err)
	}
	fmt.Printf("Service status: %+v\n", status)

	// Create or update queue
	queue, err := client.CreateOrUpdateQueue(ctx, queueName, queueTimeout)
	if err != nil {
		log.Fatalf("Error creating queue: %v", err)
	}
	fmt.Printf("Queue created: %+v\n", queue)

	// Create some example tasks
	for i := 1; i <= 3000; i++ {
		task, err := client.CreateTask(ctx, config.QueueName, map[string]interface{}{
			"job_number": i,
			"data":       fmt.Sprintf("Example data %d", i),
			"client_id":  client.ClientID(),
		})
		if err != nil {
			log.Fatalf("Error creating task: %v", err)
		}
		fmt.Printf("Task created: %+v\n", task)
	}

	// List tasks
	tasks, err := client.GetTasks(ctx, jobqueue.TaskFilter{
		QueueName: queueName,
		Limit:     10,
	})
	if err != nil {
		log.Fatalf("Error listing tasks: %v", err)
	}
	fmt.Printf("Found %d tasks\n", len(tasks))

	// Process tasks
	fmt.Println("Starting task processor...")
	err = client.ProcessTasks(ctx, config, mainTaskHandler)

	if err != nil && err != context.Canceled {
		log.Fatalf("Error processing tasks: %v", err)
	}
}

func mainTaskHandler(ctx context.Context, task *jobqueue.Task) error {
	// Decode task data
	var taskData struct {
		JobNumber int    `json:"job_number"`
		Data      string `json:"data"`
	}
	if err := json.Unmarshal(task.Data, &taskData); err != nil {
		return fmt.Errorf("error decoding task data: %w", err)
	}

	fmt.Printf("Processing task %s: Job %d - %s\n",
		task.ID, taskData.JobNumber, taskData.Data)

	// get a random number between 1 and 100
	i := 1 + rand.Int63n(4)
	fmt.Println("Processing task for", i, "seconds")

	// Simulate processing
	time.Sleep(time.Duration(i) * time.Second)
	return nil
}
