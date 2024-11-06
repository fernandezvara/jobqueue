package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/fernandezvara/jobqueues/internal/api"
	"github.com/fernandezvara/jobqueues/internal/queue"
	"github.com/fernandezvara/jobqueues/internal/storage"
)

func main() {
	// Database configuration
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgresql://postgres:postgres@localhost:5432/jobqueue?sslmode=disable"
	}

	db, err := storage.NewDB(dbURL)
	if err != nil {
		log.Fatal("failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize DB schema
	if err := storage.InitSchema(db); err != nil {
		log.Fatal("failed to initialize schema:", err)
	}

	// Init services
	store := storage.NewStore(db)
	queueService := queue.NewService(store)
	server := api.NewServer(queueService)

	// termination signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080"
		}
		log.Printf("Server starting on port %s", port)
		if err := http.ListenAndServe(":"+port, server); err != nil {
			log.Fatal(err)
		}
	}()

	// Waiting for termination signal
	<-stop
	log.Println("Shutting down...")

	// Clean up resources
	_, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	
	if err := queueService.Shutdown(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Shutdown complete")
}
