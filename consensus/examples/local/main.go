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
	"github.com/fraser/consensus/pkg/consensus/backends/file"
)

func main() {
	// Get process identity (default to hostname)
	identity := os.Getenv("INSTANCE_ID")
	if identity == "" {
		hostname, _ := os.Hostname()
		identity = hostname
	}

	// Create file backend in /tmp
	leasePath := "/tmp/consensus-lease.json"
	backend := file.NewBackend(leasePath)

	// Create manager with default config
	manager := consensus.NewManager(backend, consensus.NewConfig(identity))

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

	log.Printf("Starting leader election as %s (lease file: %s)", identity, leasePath)

	// Main work loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down")
			return
		default:
			if !lease.IsLeader() {
				log.Printf("[%s] Waiting to become leader...", identity)
				time.Sleep(2 * time.Second)
				continue
			}

			// Do leader work
			log.Printf("[%s] I am the leader! Doing work...", identity)
			doWork()
			time.Sleep(5 * time.Second)
		}
	}
}

func doWork() {
	// Simulate some work
	fmt.Println("Processing jobs...")
}
