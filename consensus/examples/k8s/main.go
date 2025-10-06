package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fraser/consensus/pkg/consensus"
	"github.com/fraser/consensus/pkg/consensus/backends/lease"
)

func main() {
	// Get pod identity from environment
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		log.Fatal("POD_NAME environment variable must be set")
	}

	// Create lease backend using environment config
	backend, err := lease.NewFromEnv("consensus-worker-leader")
	if err != nil {
		log.Fatalf("Failed to create backend: %v", err)
	}

	// Create manager with default config
	manager := consensus.NewManager(backend, consensus.NewConfig(podName))

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Received shutdown signal")
		cancel()
	}()

	// Start leader election
	lease := manager.Start(ctx)
	defer manager.Stop()

	log.Printf("Starting leader election as %s", podName)

	// Main work loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down")
			return
		default:
			if !lease.IsLeader() {
				log.Printf("[%s] Waiting to become leader...", podName)
				time.Sleep(2 * time.Second)
				continue
			}

			// Do leader work
			log.Printf("[%s] I am the leader! Doing work...", podName)
			doWork()
			time.Sleep(5 * time.Second)
		}
	}
}

func doWork() {
	// Simulate some work
	fmt.Println("Processing jobs...")
}
