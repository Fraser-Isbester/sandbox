# Leasing Package Proposal

A lightweight distributed lease package for leader election and exclusive process coordination, available in both Go and Python.

## Overview

The leasing package solves a common problem in distributed systems: ensuring that only one instance of a process is actively doing work at any given time, even when multiple copies of that process are running.

This is useful for:
- Leader election in Kubernetes deployments
- Active-passive patterns for database synchronization
- Scheduled job execution with high availability
- Any scenario where you need mutual exclusion across processes

## Design Principles

1. **Lightweight**: Minimal dependencies, simple API
2. **Pluggable**: Support for multiple storage backends
3. **Clear Interface**: Intuitive API that's easy to understand and use
4. **Production Ready**: Handles edge cases, network failures, and timeouts

## Architecture

### Core Interface

Both implementations provide the same conceptual interface:

- `Acquire(holderID, duration)` - Attempt to acquire the lease
- `Renew(holderID, duration)` - Extend the lease for the current holder
- `Release(holderID)` - Release the lease
- `IsHeld(holderID)` - Check if held by a specific holder
- `GetHolder()` - Get the current holder's ID

### Supported Backends

1. **File-based**
   - Uses JSON files with atomic operations
   - Suitable for single-node or shared filesystem scenarios
   - No external dependencies

2. **Redis**
   - Uses Redis SET with NX/EX and Lua scripts
   - Fully distributed, supports multi-node deployments
   - Requires Redis infrastructure

3. **Kubernetes Lease API**
   - Uses native Kubernetes coordination.k8s.io/v1 Lease resources
   - Perfect for Kubernetes deployments
   - No external infrastructure needed in k8s

## Implementations

### Go Implementation

Location: `leasing-go/`

```go
import (
    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
    "github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing/backends/file"
)

config := leasing.NewConfig("my-lease")
lease, _ := file.New(config, "/tmp/leases")

acquired, _ := lease.Acquire(ctx, "worker-1", 30*time.Second)
if acquired {
    // Do work...
    lease.Release(ctx, "worker-1")
}
```

**Features:**
- Native Go interfaces
- Context support for cancellation
- Comprehensive error handling
- Full examples for all backends

### Python Implementation

Location: `leasing-py/`

```python
from leasing import LeaseConfig
from leasing.backends import FileLease

config = LeaseConfig(name="my-lease", default_duration=10.0)
lease = FileLease(config, directory="/tmp/leases")

acquired = await lease.acquire("worker-1", duration=30.0)
if acquired:
    # Do work...
    await lease.release("worker-1")
```

**Features:**
- Async/await API
- Type hints throughout
- Optional dependencies for backends
- Comprehensive examples

## Use Case Example: Kubernetes Leader Election

Deploy 3 replicas of your worker application:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-worker
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: worker
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
```

Each pod runs the same code:

**Go:**
```go
lease, _ := k8s.NewInCluster(namespace, config)
acquired, _ := lease.Acquire(ctx, podName, 30*time.Second)

if acquired {
    // This pod is the leader - do work
    log.Println("I am the leader!")
    doWork()
} else {
    // This pod is a follower - stand by
    log.Println("Standing by as follower")
}
```

**Python:**
```python
lease = await K8sLease.create_in_cluster(namespace, config)
acquired = await lease.acquire(pod_name, duration=30.0)

if acquired:
    # This pod is the leader
    print("I am the leader!")
    await do_work()
else:
    # This pod is a follower
    print("Standing by as follower")
```

Result: Only 1 of the 3 pods will acquire the lease and do work. If that pod dies, another will automatically take over.

## Comparison with Existing Solutions

### vs. etcd/Consul
- **Leasing**: Simpler, pluggable backends, no dedicated infrastructure required
- **etcd/Consul**: More features, but heavier and requires dedicated servers

### vs. redis-py SETNX
- **Leasing**: Higher-level abstraction, multiple backends, proper renewal logic
- **SETNX**: Lower-level primitive, requires manual implementation

### vs. kubernetes/leader-election
- **Leasing**: Works outside k8s, multiple backends, simpler API
- **k8s leader-election**: More features but k8s-specific and more complex

## Directory Structure

```
leasing-go/
├── pkg/leasing/
│   ├── lease.go              # Core interface
│   └── backends/
│       ├── file/file.go      # File backend
│       ├── redis/redis.go    # Redis backend
│       └── k8s/k8s.go        # Kubernetes backend
├── examples/                  # Working examples
└── README.md

leasing-py/
├── leasing/
│   ├── lease.py              # Core interface
│   └── backends/
│       ├── file.py           # File backend
│       ├── redis.py          # Redis backend
│       └── k8s.py            # Kubernetes backend
├── examples/                  # Working examples
└── README.md
```

## Testing the Packages

### Go

```bash
cd leasing-go

# Test file backend
cd examples/file
go run main.go

# Test with Redis (requires Redis running)
docker run -d -p 6379:6379 redis
cd examples/redis
go run main.go

# Test with Kubernetes (requires k8s cluster)
cd examples/k8s
go run main.go
```

### Python

```bash
cd leasing-py

# Install package
pip install -e ".[all]"

# Test file backend
cd examples/file
python example.py

# Test with Redis (requires Redis running)
docker run -d -p 6379:6379 redis
cd examples/redis
python example.py

# Test with Kubernetes (requires k8s cluster)
cd examples/k8s
python example.py
```

## Next Steps

1. **Testing**: Add comprehensive unit and integration tests
2. **Observability**: Add metrics and tracing support
3. **Additional Backends**: Consider PostgreSQL, DynamoDB, etc.
4. **Advanced Features**: Lease transitions, callbacks, health checks

## Conclusion

The leasing package provides a simple, production-ready solution for distributed coordination. Its pluggable architecture allows you to start simple (file-based) and scale to distributed systems (Redis/K8s) without changing your code.

Both Go and Python implementations provide the same conceptual interface, making it easy to use across polyglot environments.
