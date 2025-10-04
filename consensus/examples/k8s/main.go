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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	// Get pod identity from environment
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		log.Fatal("POD_NAME environment variable must be set")
	}

	namespace := os.Getenv("POD_NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	// Create Kubernetes client (in-cluster config)
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get in-cluster config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	// Create lease backend
	backend := lease.NewBackend(clientset, namespace, "consensus-worker-leader", 10*time.Second)

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
