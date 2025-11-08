# Leasing (Go)

A lightweight distributed lease package for leader election and exclusive process coordination.

## Overview

The leasing package provides a simple, pluggable interface for acquiring and managing distributed leases. This enables multiple copies of a process (such as pods in Kubernetes) to run simultaneously while ensuring only one is actively doing work at any given time.

## Features

- **Lightweight**: Minimal dependencies and simple API
- **Pluggable Backends**: Support for multiple storage backends
  - File-based (for single-node or shared filesystem)
  - Redis (for distributed systems)
  - Kubernetes Lease API (native k8s support)
- **Simple Interface**: Clear, intuitive API for lease management
- **Leader Election**: Perfect for implementing active-passive patterns

## Installation

```bash
go get github.com/Fraser-Isbester/sandbox/leasing-go
```

For specific backends:

```bash
# Redis backend
go get github.com/redis/go-redis/v9

# Kubernetes backend
go get k8s.io/client-go
```

## Quick Start

### File-based Lease

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing/backends/file"
)

func main() {
    config := leasing.NewConfig("my-worker-lease")
    lease, _ := file.New(config, "/tmp/leases")

    ctx := context.Background()
    holderID := "worker-1"

    // Acquire the lease
    acquired, _ := lease.Acquire(ctx, holderID, 30*time.Second)
    if acquired {
        fmt.Println("✓ Acquired lease, doing work...")

        // Do your work here
        doWork()

        // Release when done
        lease.Release(ctx, holderID)
    }
}
```

### Redis-based Lease

```go
package main

import (
    "context"
    "time"

    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing/backends/redis"
    goredis "github.com/redis/go-redis/v9"
)

func main() {
    client := goredis.NewClient(&goredis.Options{
        Addr: "localhost:6379",
    })
    defer client.Close()

    config := leasing.NewConfig("my-worker-lease")
    lease, _ := redis.New(client, config)

    ctx := context.Background()
    holderID := "worker-1"

    acquired, _ := lease.Acquire(ctx, holderID, 30*time.Second)
    if acquired {
        // Do work...
        lease.Release(ctx, holderID)
    }
}
```

### Kubernetes Lease API

```go
package main

import (
    "context"
    "os"
    "time"

    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing/backends/k8s"
)

func main() {
    namespace := os.Getenv("NAMESPACE")
    podName := os.Getenv("POD_NAME")

    config := leasing.NewConfig("my-worker-lease")
    lease, _ := k8s.NewInCluster(namespace, config)

    ctx := context.Background()
    acquired, _ := lease.Acquire(ctx, podName, 30*time.Second)

    if acquired {
        // This pod is the leader
        // Do work...
        lease.Release(ctx, podName)
    } else {
        // Another pod is the leader, stand by
    }
}
```

## API Reference

### Lease Interface

```go
type Lease interface {
    // Acquire attempts to acquire the lease
    Acquire(ctx context.Context, holderID string, duration time.Duration) (bool, error)

    // Renew extends the lease duration
    Renew(ctx context.Context, holderID string, duration time.Duration) error

    // Release explicitly releases the lease
    Release(ctx context.Context, holderID string) error

    // IsHeld checks if the lease is held by the specified holder
    IsHeld(holderID string) bool

    // GetHolder returns the current holder's ID
    GetHolder(ctx context.Context) (string, error)
}
```

### Configuration

```go
type Config struct {
    Name            string        // Unique name for this lease
    DefaultDuration time.Duration // Default lease duration
}

config := leasing.NewConfig("my-lease")
config.DefaultDuration = 15 * time.Second
```

## Use Cases

### Leader Election in Kubernetes

Deploy multiple replicas of your application, but ensure only one is active:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-worker
spec:
  replicas: 3  # Multiple replicas for HA
  template:
    spec:
      containers:
      - name: worker
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

Each pod will try to acquire the lease, but only one will succeed and do work.

### Active-Passive Database Sync

Multiple sync processes can run, but only one actively syncs at a time:

```go
func runSync() {
    lease, _ := redis.New(client, config)

    ticker := time.NewTicker(5 * time.Second)
    for range ticker.C {
        acquired, _ := lease.Acquire(ctx, workerID, 10*time.Second)
        if acquired {
            syncDatabase()
            lease.Renew(ctx, workerID, 10*time.Second)
        }
    }
}
```

### Scheduled Jobs with High Availability

Multiple job schedulers can run, but only one executes jobs:

```go
func jobScheduler() {
    lease, _ := file.New(config, "/shared/leases")

    for job := range jobs {
        acquired, _ := lease.Acquire(ctx, workerID, 30*time.Second)
        if acquired {
            executeJob(job)
            lease.Release(ctx, workerID)
        }
    }
}
```

## Examples

See the `examples/` directory for complete working examples:

- `examples/file/` - File-based lease example
- `examples/redis/` - Redis-based lease example
- `examples/k8s/` - Kubernetes Lease API example

Run an example:

```bash
cd examples/file
go run main.go
```

## Backend Comparison

| Backend | Use Case | Pros | Cons |
|---------|----------|------|------|
| **File** | Single-node, shared filesystem | Simple, no dependencies | Not suitable for true distributed systems |
| **Redis** | Distributed systems | Fast, reliable, widely used | Requires Redis infrastructure |
| **K8s Lease** | Kubernetes deployments | Native k8s integration, no external deps | Only works in Kubernetes |

## Best Practices

1. **Always release leases**: Use `defer` to ensure leases are released
2. **Set appropriate durations**: Balance between responsiveness and safety
3. **Handle renewal failures**: Plan for network issues and timeouts
4. **Use unique holder IDs**: Ensure each process has a unique identifier
5. **Monitor lease holders**: Log who holds the lease for debugging

## License

MIT
