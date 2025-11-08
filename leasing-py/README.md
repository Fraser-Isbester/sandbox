# Leasing (Python)

A lightweight distributed lease package for leader election and exclusive process coordination.

## Overview

The leasing package provides a simple, pluggable interface for acquiring and managing distributed leases. This enables multiple copies of a process (such as pods in Kubernetes) to run simultaneously while ensuring only one is actively doing work at any given time.

## Features

- **Lightweight**: Minimal dependencies and simple async API
- **Pluggable Backends**: Support for multiple storage backends
  - File-based (for single-node or shared filesystem)
  - Redis (for distributed systems)
  - Kubernetes Lease API (native k8s support)
- **Simple Interface**: Clear, intuitive async/await API
- **Leader Election**: Perfect for implementing active-passive patterns

## Installation

```bash
pip install ./leasing-py
```

For specific backends:

```bash
# Redis backend
pip install ".[redis]"

# Kubernetes backend
pip install ".[k8s]"

# All backends
pip install ".[all]"

# Development dependencies
pip install ".[dev]"
```

## Quick Start

### File-based Lease

```python
import asyncio
from leasing import LeaseConfig
from leasing.backends import FileLease

async def main():
    config = LeaseConfig(name="my-worker-lease", default_duration=10.0)
    lease = FileLease(config, directory="/tmp/leases")

    holder_id = "worker-1"

    # Acquire the lease
    acquired = await lease.acquire(holder_id, duration=30.0)
    if acquired:
        print("✓ Acquired lease, doing work...")

        # Do your work here
        await do_work()

        # Release when done
        await lease.release(holder_id)

asyncio.run(main())
```

### Redis-based Lease

```python
import asyncio
from leasing import LeaseConfig
from leasing.backends import RedisLease

async def main():
    config = LeaseConfig(name="my-worker-lease", default_duration=10.0)
    lease = await RedisLease.create("redis://localhost:6379", config)

    holder_id = "worker-1"

    try:
        acquired = await lease.acquire(holder_id, duration=30.0)
        if acquired:
            # Do work...
            await lease.release(holder_id)
    finally:
        await lease.close()

asyncio.run(main())
```

### Kubernetes Lease API

```python
import asyncio
import os
from leasing import LeaseConfig
from leasing.backends import K8sLease

async def main():
    namespace = os.getenv("NAMESPACE", "default")
    pod_name = os.getenv("POD_NAME")

    config = LeaseConfig(name="my-worker-lease", default_duration=15.0)
    lease = await K8sLease.create_in_cluster(namespace, config)

    acquired = await lease.acquire(pod_name, duration=30.0)

    if acquired:
        # This pod is the leader
        # Do work...
        await lease.release(pod_name)
    else:
        # Another pod is the leader, stand by
        pass

asyncio.run(main())
```

## API Reference

### Lease Interface

```python
class Lease(ABC):
    async def acquire(self, holder_id: str, duration: Optional[float] = None) -> bool:
        """Attempt to acquire the lease"""

    async def renew(self, holder_id: str, duration: Optional[float] = None) -> None:
        """Extend the lease duration"""

    async def release(self, holder_id: str) -> None:
        """Release the lease"""

    def is_held(self, holder_id: str) -> bool:
        """Check if lease is held by the specified holder"""

    async def get_holder(self) -> Optional[str]:
        """Get the current holder's ID"""
```

### Configuration

```python
from leasing import LeaseConfig

config = LeaseConfig(
    name="my-lease",
    default_duration=15.0  # seconds
)
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

```python
async def run_sync():
    lease = await RedisLease.create("redis://localhost:6379", config)

    while True:
        acquired = await lease.acquire(worker_id, duration=10.0)
        if acquired:
            await sync_database()
            await lease.renew(worker_id, duration=10.0)

        await asyncio.sleep(5)
```

### Scheduled Jobs with High Availability

Multiple job schedulers can run, but only one executes jobs:

```python
async def job_scheduler():
    lease = FileLease(config, directory="/shared/leases")

    for job in jobs:
        acquired = await lease.acquire(worker_id, duration=30.0)
        if acquired:
            await execute_job(job)
            await lease.release(worker_id)
```

## Examples

See the `examples/` directory for complete working examples:

- `examples/file/` - File-based lease example
- `examples/redis/` - Redis-based lease example
- `examples/k8s/` - Kubernetes Lease API example

Run an example:

```bash
cd examples/file
python example.py
```

## Backend Comparison

| Backend | Use Case | Pros | Cons |
|---------|----------|------|------|
| **File** | Single-node, shared filesystem | Simple, no dependencies | Not suitable for true distributed systems |
| **Redis** | Distributed systems | Fast, reliable, widely used | Requires Redis infrastructure |
| **K8s Lease** | Kubernetes deployments | Native k8s integration, no external deps | Only works in Kubernetes |

## Best Practices

1. **Always release leases**: Use try/finally to ensure leases are released
2. **Set appropriate durations**: Balance between responsiveness and safety
3. **Handle renewal failures**: Plan for network issues and timeouts
4. **Use unique holder IDs**: Ensure each process has a unique identifier
5. **Monitor lease holders**: Log who holds the lease for debugging
6. **Use async/await**: This library is built for asyncio - embrace it

## Error Handling

```python
from leasing.lease import LeaseNotHeldError

try:
    await lease.renew(holder_id)
except LeaseNotHeldError:
    # We lost the lease, handle gracefully
    print("Lost lease, stopping work")
```

## Development

```bash
# Install development dependencies
pip install ".[dev]"

# Run tests
pytest

# Format code
black leasing/

# Type checking
mypy leasing/
```

## License

MIT
