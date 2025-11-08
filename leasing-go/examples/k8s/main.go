package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
	"github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing/backends/k8s"
)

func main() {
	// Get namespace and pod name from environment (typically set by k8s)
	namespace := os.Getenv("NAMESPACE")
	if namespace == "" {
		namespace = "default"
	}

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		podName = fmt.Sprintf("local-%d", os.Getpid())
	}

	// Create a lease configuration
	config := leasing.NewConfig("my-worker-lease")
	config.DefaultDuration = 15 * time.Second

	// Create a Kubernetes Lease API-based lease
	// This will use in-cluster configuration when running in a pod
	lease, err := k8s.NewInCluster(namespace, config)
	if err != nil {
		log.Fatalf("Failed to create lease: %v", err)
	}

	ctx := context.Background()

	// Try to acquire the lease
	fmt.Printf("Pod %s attempting to acquire lease...\n", podName)
	acquired, err := lease.Acquire(ctx, podName, 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to acquire lease: %v", err)
	}

	if acquired {
		fmt.Printf("✓ Pod %s acquired the lease!\n", podName)

		// Do work while holding the lease
		fmt.Println("Doing work as leader...")
		doWork(ctx, lease, podName)

		// Release the lease when done
		fmt.Println("Releasing lease...")
		if err := lease.Release(ctx, podName); err != nil {
			log.Printf("Failed to release lease: %v", err)
		}
		fmt.Println("✓ Lease released")
	} else {
		fmt.Printf("✗ Lease is held by another pod\n")

		// Check who holds it
		holder, err := lease.GetHolder(ctx)
		if err != nil {
			log.Printf("Failed to get holder: %v", err)
		} else if holder != "" {
			fmt.Printf("Current leader: %s\n", holder)
		}

		// In a real app, you'd loop and keep trying, or just wait
		fmt.Println("Standing by as follower...")
	}
}

// doWork simulates work while holding the lease, with periodic renewals
func doWork(ctx context.Context, lease leasing.Lease, holderID string) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	workDone := time.After(20 * time.Second)

	for {
		select {
		case <-workDone:
			fmt.Println("Work completed!")
			return

		case <-ticker.C:
			// Renew the lease periodically
			if err := lease.Renew(ctx, holderID, 30*time.Second); err != nil {
				log.Printf("Failed to renew lease: %v", err)
				return
			}
			fmt.Println("✓ Lease renewed, still leading")
		}
	}
}
