package jobqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// ProcessTasks processes tasks from the queue concurrently
func (c *Client) ProcessTasks(ctx context.Context, config ProcessTasksConfig, processor func(context.Context, *Task) error) error {

	if config.WorkerCount < 1 {
		config.WorkerCount = 1
	}

	// Get queue information to know the timeout
	queue, err := c.GetQueue(ctx, config.QueueName)
	if err != nil {
		return fmt.Errorf("error getting queue info: %w", err)
	}
	if queue == nil {
		return fmt.Errorf("queue %s does not exist", config.QueueName)
	}

	// Channel to distribute tasks to workers
	tasksChan := make(chan *Task, config.WorkerBuffer)
	// Channel to receive results from workers
	resultsChan := make(chan taskResult, config.WorkerBuffer)
	// Channel to signal critical errors
	errorsChan := make(chan error, 1)
	// WaitGroup to wait for all workers to finish
	var wg sync.WaitGroup

	// Cancelable context for workers
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start workers
	for i := 0; i < config.WorkerCount; i++ {
		wg.Add(1)
		go c.runWorker(workerCtx, &wg, queue.TaskTimeout, tasksChan, resultsChan, processor)
	}

	// Goroutine to process results
	go func() {
		for result := range resultsChan {
			if err := c.handleTaskResult(workerCtx, result, config); err != nil {
				if config.StopOnError {
					errorsChan <- err
					cancel()
					return
				}
				log.Printf("Error handling task result: %v", err)
			}
		}
	}()

	// Main loop to get tasks
	for {
		select {
		case <-workerCtx.Done():
			close(tasksChan)
			wg.Wait()
			close(resultsChan)
			return workerCtx.Err()

		case err := <-errorsChan:
			close(tasksChan)
			wg.Wait()
			close(resultsChan)
			return err

		default:
			task, err := c.GetNextTask(workerCtx, config.QueueName)
			if err != nil {
				if config.StopOnError {
					errorsChan <- fmt.Errorf("error getting next task: %w", err)
					continue
				}
				log.Printf("Error getting next task: %v, retrying in %v", err, config.RetryInterval)
				time.Sleep(config.RetryInterval)
				continue
			}

			if task == nil {
				time.Sleep(config.RetryInterval)
				continue
			}

			select {
			case tasksChan <- task:
				// Task sent to worker
			case <-workerCtx.Done():
				return workerCtx.Err()
			}
		}
	}
}

func (c *Client) runWorker(ctx context.Context, wg *sync.WaitGroup, timeout time.Duration,
	tasks <-chan *Task, results chan<- taskResult, processor func(context.Context, *Task) error) {
	defer wg.Done()

	for task := range tasks {
		// Create context with timeout for the task
		taskCtx, cancel := context.WithTimeout(ctx, timeout)

		// Channel for the processing result
		done := make(chan error, 1)

		// Process the task
		go func() {
			done <- processor(taskCtx, task)
		}()

		// Wait for result or timeout
		var processingErr error
		select {
		case err := <-done:
			processingErr = err
		case <-taskCtx.Done():
			if taskCtx.Err() == context.DeadlineExceeded {
				processingErr = fmt.Errorf("task processing exceeded timeout of %v", timeout)
			} else {
				processingErr = taskCtx.Err()
			}
		}

		// Clean up the context
		cancel()

		// Send result
		results <- taskResult{
			task: task,
			err:  processingErr,
			data: task.Data,
		}

		if ctx.Err() != nil {
			return
		}
	}
}

// handleTaskResult handles the result of a processed task
func (c *Client) handleTaskResult(ctx context.Context, result taskResult, config ProcessTasksConfig) error {
	var updatedData json.RawMessage
	if result.err != nil {
		errorData := make(map[string]interface{})

		if result.data != nil && config.PreserveError {
			if err := json.Unmarshal(result.data, &errorData); err == nil {
				errorData["error"] = result.err.Error()
			} else {
				errorData = map[string]interface{}{
					"original_data": string(result.data),
					"error":         result.err.Error(),
				}
			}
		} else {
			errorData = map[string]interface{}{
				"error": result.err.Error(),
			}
		}

		updatedDataBytes, err := json.Marshal(errorData)
		if err != nil {
			updatedDataBytes = []byte(fmt.Sprintf(`{"error":"Error encoding task data: %v"}`, err))
		}
		updatedData = updatedDataBytes
	} else {
		updatedData = result.data
	}

	status := "completed"
	if result.err != nil {
		status = "failed"
	}

	_, err := c.UpdateTask(ctx, result.task.ID, status, updatedData)
	return err
}

// func (c *Client) ProcessTasks(ctx context.Context, config ProcessTasksConfig, processor func(context.Context, *Task) error) error {
// 	// if c.clientID == "" {
// 	// 	return fmt.Errorf("client ID is required for processing tasks")
// 	// }

// 	// Get queue information to know the timeout
// 	queue, err := c.GetQueue(ctx, config.QueueName)
// 	if (err != nil) {
// 		return fmt.Errorf("error getting queue info: %w", err)
// 	}
// 	if queue == nil {
// 		return fmt.Errorf("queue %s does not exist", config.QueueName)
// 	}

// 	for {
// 		select {
// 		case <-ctx.Done():
// 			return ctx.Err()
// 		default:
// 			task, err := c.GetNextTask(ctx, config.QueueName)
// 			if err != nil {
// 				if config.StopOnError {
// 					return fmt.Errorf("error getting next task: %w", err)
// 				}
// 				log.Printf("Error getting next task: %v, retrying in %v", err, config.RetryInterval)
// 				time.Sleep(config.RetryInterval)
// 				continue
// 			}

// 			if task == nil {
// 				time.Sleep(config.RetryInterval)
// 				continue
// 			}

// 			// Create a context with timeout for processing
// 			taskCtx, cancel := context.WithTimeout(ctx, queue.TaskTimeout)

// 			// Channel to detect the end of processing
// 			done := make(chan error, 1)

// 			// Process the task in a goroutine
// 			go func() {
// 				done <- processor(taskCtx, task)
// 			}()

// 			// Wait for processing to finish or timeout to be exceeded
// 			var processingErr error
// 			select {
// 			case err := <-done:
// 				processingErr = err
// 			case <-taskCtx.Done():
// 				if taskCtx.Err() == context.DeadlineExceeded {
// 					processingErr = fmt.Errorf("task processing exceeded timeout of %v", queue.TaskTimeout)
// 				} else {
// 					processingErr = taskCtx.Err()
// 				}
// 			}

// 			// Clean up the context
// 			cancel()

// 			// Prepare updated task data
// 			var updatedData json.RawMessage
// 			if processingErr != nil {
// 				errorData := make(map[string]interface{})

// 				// If there is existing data and it is a valid JSON object, preserve it
// 				if task.Data != nil && config.PreserveError {
// 					if err := json.Unmarshal(task.Data, &errorData); err == nil {
// 						// The existing data was a valid JSON object
// 						errorData["error"] = processingErr.Error()
// 					} else {
// 						// The data was not a valid JSON object, create a new one
// 						errorData = map[string]interface{}{
// 							"original_data": string(task.Data),
// 							"error":         processingErr.Error(),
// 						}
// 					}
// 				} else {
// 					// There was no previous data or we do not want to preserve it
// 					errorData = map[string]interface{}{
// 						"error": processingErr.Error(),
// 					}
// 				}

// 				updatedDataBytes, err := json.Marshal(errorData)
// 				if err != nil {
// 					updatedDataBytes = []byte(fmt.Sprintf(`{"error":"Error encoding task data: %v"}`, err))
// 				}
// 				updatedData = updatedDataBytes
// 			} else {
// 				updatedData = task.Data
// 			}

// 			// Update the task status
// 			status := "completed"
// 			if processingErr != nil {
// 				status = "failed"
// 			}

// 			_, err = c.UpdateTask(ctx, task.ID, status, updatedData)
// 			if err != nil {
// 				if config.StopOnError {
// 					return fmt.Errorf("error updating task status: %w", err)
// 				}
// 				log.Printf("Error updating task status: %v", err)
// 			}

// 			if processingErr != nil && config.StopOnError {
// 				return fmt.Errorf("task processing error: %w", processingErr)
// 			}
// 		}
// 	}
// }
