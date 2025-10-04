# Consensus

A Go package for distributed leader election across N processes using pluggable storage backends.

## Features

- **Pluggable backends**: Kubernetes Lease API, file-based (local testing)
- **Simple API**: Start election with a few lines of code
- **Thread-safe**: Safe for concurrent use
- **Graceful shutdown**: Automatic leadership release on context cancellation

## Installation

```bash
go get github.com/fraser/consensus
```

## Backends

### Kubernetes Lease Backend

Production-ready backend using Kubernetes Lease objects for leader election in cluster environments.

```go
import (
    "github.com/fraser/consensus/pkg/consensus"
    "github.com/fraser/consensus/pkg/consensus/backends/lease"
)

backend := lease.NewBackend(clientset, "default", "my-app-leader", 15*time.Second)
manager := consensus.NewManager(backend, consensus.NewConfig(podName))
```

**Required RBAC:**
```yaml
apiGroups: ["coordination.k8s.io"]
resources: ["leases"]
verbs: ["get", "create", "update"]
```

### File Backend

File-based backend for local development and testing. Uses file locking for atomic operations.

```go
import (
    "github.com/fraser/consensus/pkg/consensus"
    "github.com/fraser/consensus/pkg/consensus/backends/file"
)

backend := file.NewBackend("/tmp/leader.json", 15*time.Second)
manager := consensus.NewManager(backend, consensus.NewConfig("instance-1"))
```

## Usage

### Basic Pattern

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/fraser/consensus/pkg/consensus"
    "github.com/fraser/consensus/pkg/consensus/backends/file"
)

func main() {
    backend := file.NewBackend("/tmp/leader.json", 15*time.Second)
    manager := consensus.NewManager(backend, consensus.NewConfig("instance-1"))

    ctx := context.Background()
    lease := manager.Start(ctx)
    defer manager.Stop()

    for {
        if !lease.IsLeader() {
            log.Println("Waiting to become leader...")
            time.Sleep(2 * time.Second)
            continue
        }

        log.Println("I am the leader!")
        doWork()
        time.Sleep(5 * time.Second)
    }
}

func doWork() {
    // Your work here
}
```

### Wait-Based Pattern

```go
for {
    if err := lease.WaitForLeadership(ctx); err != nil {
        return err
    }

    for lease.IsLeader() {
        doWork()
        time.Sleep(2 * time.Second)
    }
}
```

## Configuration

### Default Configuration

```go
config := consensus.NewConfig("my-identity")
// Defaults:
// - LeaseDuration: 15s
// - RenewInterval: 5s
// - RetryInterval: 2s
```

### Custom Configuration

```go
config := consensus.Config{
    Identity:      "my-identity",
    LeaseDuration: 30 * time.Second,
    RenewInterval: 10 * time.Second,
    RetryInterval: 5 * time.Second,
}
```

## Testing in Kubernetes

### Build and Deploy

```bash
# Build Docker image
docker build -t consensus:latest .

# Load into Kind cluster (if using Kind)
kind load docker-image consensus:latest

# Deploy to Kubernetes
kubectl apply -f k8s/deployment.yaml

# Watch the logs
kubectl logs -f -l app=consensus --all-containers=true
```

You should see leader election in action with only one pod claiming leadership at a time.

### Local Testing

Run multiple instances locally using the file backend:

```bash
# Terminal 1
INSTANCE_ID=instance-1 go run examples/local/main.go

# Terminal 2
INSTANCE_ID=instance-2 go run examples/local/main.go

# Terminal 3
INSTANCE_ID=instance-3 go run examples/local/main.go
```

Only one instance will claim leadership. Kill the leader to see automatic failover.

## How It Works

1. **Leader**: Periodically renews lease using `RenewInterval`
2. **Non-leader**: Periodically attempts to acquire leadership using `RetryInterval`
3. **Expiry**: If leader fails to renew within `LeaseDuration`, lease expires and others can acquire
4. **Fault tolerance**: Transient renewal failures are tolerated; consecutive failures demote the leader

## API Reference

### Manager

- `NewManager(backend Backend, config Config) *Manager` - Create new manager
- `Start(ctx context.Context) *Lease` - Start leader election
- `Stop() error` - Stop election and release leadership

### Lease

- `IsLeader() bool` - Check leadership status (non-blocking)
- `WaitForLeadership(ctx context.Context) error` - Block until becoming leader

### Backend Interface

```go
type Backend interface {
    TryAcquire(ctx context.Context, identity string) (bool, error)
    Renew(ctx context.Context, identity string) error
    Release(ctx context.Context, identity string) error
}
```

## License

MIT
