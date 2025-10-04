# Backend NewFromEnv() Specification

## Overview
Each backend implements a `NewFromEnv()` constructor that reads configuration from environment variables, providing opinionated defaults for common deployment scenarios.

## Lease Backend

**Path:** `backends/lease/lease.go`

```go
// NewFromEnv creates a Lease backend using in-cluster Kubernetes config
// leaseName: name of the Lease object to use for coordination
// Environment variables:
//   POD_NAMESPACE - namespace for lease object (default: "default")
//   CONSENSUS_LEASE_TTL - lease duration in seconds (default: "15")
func NewFromEnv(leaseName string) (*Backend, error)
```

**Implementation:**
1. Read `POD_NAMESPACE`, default to `"default"`
2. Read `CONSENSUS_LEASE_TTL`, parse as int seconds, default to 15
4. Get in-cluster config via `rest.InClusterConfig()`
5. Create clientset
6. Return `NewBackend(clientset, namespace, name, ttl)`

**Error cases:**
- Failed to get in-cluster config (not running in k8s)
- Failed to create clientset
- Invalid TTL value (not parseable as int)

## Redis Backend

**Path:** `backends/redis/redis.go`

```go
// NewFromEnv creates a Redis backend from environment variables
// key: Redis key name for the lock
// Environment variables:
//   REDIS_ADDR - Redis server address (default: "localhost:6379")
//   REDIS_PASSWORD - Redis password (default: "")
//   REDIS_DB - Redis database number (default: "0")
//   CONSENSUS_REDIS_TTL - lock duration in seconds (default: "15")
func NewFromEnv(key string) (*Backend, error)
```

**Implementation:**
1. Read Redis connection params, use defaults if not set
2. Read `CONSENSUS_REDIS_TTL`, parse as int seconds, default to 15
4. Create `redis.Client` with options
5. Test connection with `Ping()`
6. Return `NewBackend(client, key, ttl)`

**Error cases:**
- Failed to connect to Redis
- Invalid TTL value
- Invalid DB number

## File Backend

**Path:** `backends/file/file.go`

```go
// NewFromEnv creates a File backend from environment variables
// path: filesystem path to the lock file
// Environment variables:
//   CONSENSUS_FILE_TTL - lock duration in seconds (default: "15")
func NewFromEnv(path string) (*Backend, error)
```

**Implementation:**
1. Read `CONSENSUS_FILE_TTL`, parse as int seconds, default to 15
3. Ensure parent directory exists (create if needed)
4. Return `NewBackend(path, ttl)`

**Error cases:**
- Invalid TTL value
- Cannot create parent directory
- Path not writable

## Usage Pattern

```go
// Development - explicit config
backend := lease.NewBackend(clientset, "my-ns", "my-lease", 30*time.Second)

// Production - environment-driven
backend, err := lease.NewFromEnv("my-app-leader")
if err != nil {
    log.Fatal(err)
}

// Both use same Manager
manager := consensus.NewManager(backend, consensus.NewConfig(os.Getenv("POD_NAME")))
```

## Documentation Requirements

Each `NewFromEnv()` must document:
- All supported environment variables
- Default values for each
- Expected format/validation rules
- Common error scenarios
