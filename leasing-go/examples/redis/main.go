package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
	"github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing/backends/redis"
	goredis "github.com/redis/go-redis/v9"
)

func main() {
	// Get Redis address from environment or use default
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// Create Redis client
	client := goredis.NewClient(&goredis.Options{
		Addr: redisAddr,
	})
	defer client.Close()

	// Verify connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Create a lease configuration
	config := leasing.NewConfig("my-worker-lease")
	config.DefaultDuration = 10 * time.Second

	// Create a Redis-based lease
	lease, err := redis.New(client, config)
	if err != nil {
		log.Fatalf("Failed to create lease: %v", err)
	}

	// Get our identity
	holderID := os.Getenv("WORKER_ID")
	if holderID == "" {
		holderID = fmt.Sprintf("worker-%d", os.Getpid())
	}

	// Try to acquire the lease
	fmt.Printf("Worker %s attempting to acquire lease...\n", holderID)
	acquired, err := lease.Acquire(ctx, holderID, 30*time.Second)
	if err != nil {
		log.Fatalf("Failed to acquire lease: %v", err)
	}

	if acquired {
		fmt.Printf("✓ Worker %s acquired the lease!\n", holderID)

		// Do work while holding the lease
		fmt.Println("Doing work...")
		doWork(ctx, lease, holderID)

		// Release the lease when done
		fmt.Println("Releasing lease...")
		if err := lease.Release(ctx, holderID); err != nil {
			log.Printf("Failed to release lease: %v", err)
		}
		fmt.Println("✓ Lease released")
	} else {
		fmt.Printf("✗ Lease is held by another worker\n")

		// Check who holds it
		holder, err := lease.GetHolder(ctx)
		if err != nil {
			log.Printf("Failed to get holder: %v", err)
		} else if holder != "" {
			fmt.Printf("Current holder: %s\n", holder)
		}
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
			fmt.Println("✓ Lease renewed")
		}
	}
}
