package queue

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fernandezvara/jobqueues/internal/storage"
)

type TimeoutWorker struct {
	store    storage.Store
	interval time.Duration
	stopChan chan struct{}
	doneChan chan struct{}
}

func NewTimeoutWorker(store storage.Store, checkInterval time.Duration) *TimeoutWorker {
	return &TimeoutWorker{
		store:    store,
		interval: checkInterval,
		stopChan: make(chan struct{}),
		doneChan: make(chan struct{}),
	}
}

func (w *TimeoutWorker) Start() {
	go w.run()
}

func (w *TimeoutWorker) Stop() {
	close(w.stopChan)
	<-w.doneChan
}

func (w *TimeoutWorker) run() {
	defer close(w.doneChan)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopChan:
			return
		case <-ticker.C:
			fmt.Println("Checking for expired tasks...")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := w.store.MarkExpiredTasks(ctx); err != nil {
				log.Printf("Error marking expired tasks: %v", err)
			}
			cancel()
		}
	}
}
