package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/pubsub"
	_ "github.com/lib/pq"

	"github.com/fraser-isbester/pubsubscheduler/pubsubscheduler"
)

func main() {
	// Get configuration from environment
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	projectID := os.Getenv("PUBSUB_PROJECT_ID")
	if projectID == "" {
		log.Fatal("PUBSUB_PROJECT_ID environment variable is required")
	}

	workerID := os.Getenv("WORKER_ID")
	if workerID == "" {
		workerID = os.Getenv("HOSTNAME") // Use pod name in Kubernetes
	}

	// Connect to database
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}
	log.Println("Connected to database")

	// Create Pub/Sub client
	ctx := context.Background()
	pubsubClient, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("Failed to create Pub/Sub client: %v", err)
	}
	defer pubsubClient.Close()
	log.Println("Connected to Pub/Sub")

	// Create scheduler
	scheduler := pubsubscheduler.NewScheduler(db, pubsubClient, workerID)

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start worker in a goroutine
	go func() {
		opts := pubsubscheduler.DefaultWorkerOptions()
		if err := scheduler.RunWorker(ctx, opts); err != nil && err != context.Canceled {
			log.Printf("Worker error: %v", err)
		}
	}()

	// Start schedule processor in a goroutine
	go func() {
		if err := scheduler.RunScheduleProcessor(ctx, 10*time.Second); err != nil && err != context.Canceled {
			log.Printf("Schedule processor error: %v", err)
		}
	}()

	log.Println("Worker and schedule processor started, waiting for shutdown signal...")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received, stopping...")

	cancel()
	time.Sleep(2 * time.Second) // Give workers time to finish current batch

	log.Println("Shutdown complete")
}
