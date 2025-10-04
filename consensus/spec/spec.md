# Consensus Package Specification

## Overview
Build a Go package for distributed leader election across N processes using pluggable storage backends. Only one process should execute work at any time.

## Package Structure
```
pkg/consensus/
├── consensus.go          # Core types and Manager
├── backend.go            # Backend interface
├── backends/
│   ├── lease/
│   │   └── lease.go      # Kubernetes Lease backend
│   └── file/
│       └── file.go       # File-based backend (local/CI testing)
```

## Core API

### Backend Interface
```go
// Backend abstracts the storage mechanism for leader election
type Backend interface {
    // TryAcquire attempts to acquire or renew leadership
    // Returns true if leadership was acquired/renewed, false if another leader holds it
    TryAcquire(ctx context.Context, identity string) (bool, error)

    // Renew extends the current leader's lease
    // Returns error if we're no longer the leader
    Renew(ctx context.Context, identity string) error

    // Release explicitly gives up leadership
    Release(ctx context.Context, identity string) error
}
```

### Manager Config
```go
type Config struct {
    Identity      string        // Unique identifier for this instance (e.g., POD_NAME)
    LeaseDuration time.Duration // How long a lease is valid before expiring
    RenewInterval time.Duration // How often the leader renews its lease
    RetryInterval time.Duration // How often non-leaders retry acquiring leadership
}

// NewConfig creates a Config with sensible defaults
// Defaults: LeaseDuration=15s, RenewInterval=5s, RetryInterval=2s
func NewConfig(identity string) Config
```

### Manager
```go
type Manager struct {
    backend Backend
    config  Config
    // Internal: goroutine management, state tracking
}

// NewManager creates a new leader election manager
func NewManager(backend Backend, config Config) *Manager

// Start begins the leader election process
// Returns immediately with a Lease for checking leadership status
func (m *Manager) Start(ctx context.Context) *Lease

// Stop gracefully stops leader election and releases leadership if held
func (m *Manager) Stop() error
```

### Lease
```go
type Lease struct {
    // Internal: reference to manager, leadership channel
}

// IsLeader returns true if this instance is currently the leader
// Non-blocking, safe to call in tight loops
func (l *Lease) IsLeader() bool

// WaitForLeadership blocks until this instance becomes leader or context cancels
func (l *Lease) WaitForLeadership(ctx context.Context) error
```

## Implementation Requirements

### Manager.run() Internal Loop
1. On tick interval:
   - If leader: call `backend.Renew()`, use `RenewInterval`
     - Single Renew failure: keep leadership, retry Renew on next tick
     - Consecutive Renew failures (e.g., 2-3): demote to non-leader
   - If non-leader: call `backend.TryAcquire()`, use `RetryInterval`
2. Track leadership state changes:
   - Gained leadership: close `lease.leader` channel (signals waiters)
   - Lost leadership: recreate `lease.leader` channel (makes `IsLeader()` return false)
3. On context cancellation:
   - Call `backend.Release()` if currently leader
   - Return gracefully

### Lease Backend: K8s Lease Objects

**Path:** `backends/lease/lease.go`

```go
type Backend struct {
    client    kubernetes.Interface
    namespace string
    name      string
    ttl       time.Duration
}

func NewBackend(client kubernetes.Interface, namespace, name string, ttl time.Duration) *Backend
```

**TryAcquire logic:**
1. GET lease object
2. If NotFound: CREATE with current identity as holder
3. If exists:
   - If we're already the holder: UPDATE renewTime to now, return true
   - If different holder: check if `now - renewTime > ttl`
     - Expired: UPDATE to our identity, return true
     - Valid: return false (can't acquire)
4. Handle conflict errors (retry in Manager loop)

**Renew logic:**
1. GET lease object
2. Verify `holderIdentity == our identity`, else return error
3. UPDATE renewTime to now

**Release logic:**
1. GET lease object
2. If `holderIdentity == our identity`: UPDATE to set holderIdentity to empty/nil

**Lease object fields used:**
- `spec.holderIdentity`: current leader's identity
- `spec.renewTime`: last time leader renewed (used for expiry check)
- `spec.leaseDurationSeconds`: TTL value
- `spec.acquireTime`: when current holder first acquired (informational)

### File Backend (Local Testing)

**Path:** `backends/file/file.go`

```go
type Backend struct {
    path string
    ttl  time.Duration
}

func NewBackend(path string, ttl time.Duration) *Backend
```

**Implementation:**
- Uses a file with JSON content: `{"holder": "identity", "renewTime": "RFC3339"}`
- File locking (flock/syscall) for atomic read-modify-write operations
- TryAcquire: read file, check expiry, write if can acquire
- Renew: read file, verify holder, update renewTime
- Release: read file, verify holder, clear holder field

**Thread safety:** File locking ensures atomicity across processes

## Usage Examples

### Basic Leader Election
```go
backend := lease.NewBackend(clientset, "default", "my-app-leader", 5*time.Second)

manager := consensus.NewManager(backend, consensus.NewConfig(os.Getenv("POD_NAME")))

// Or explicit config:
// manager := consensus.NewManager(backend, consensus.Config{
//     Identity:      os.Getenv("POD_NAME"),
//     LeaseDuration: 5 * time.Second,
//     RenewInterval: 1 * time.Second,
//     RetryInterval: 2 * time.Second,
// })

ctx, cancel := context.WithCancel(context.Background())
defer cancel()

lease := manager.Start(ctx)
defer manager.Stop()

for {
    if !lease.IsLeader() {
        time.Sleep(time.Second)
        continue
    }

    // Do leader work
    processJobs()
    time.Sleep(2 * time.Second)
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

## Testing Requirements

1. **Unit tests** for each backend:
   - Single instance acquires leadership
   - Second instance blocked while first holds lease
   - Lease expiry allows takeover
   - Renewal extends lease
   - Release allows immediate takeover

2. **Integration test** with multiple goroutines simulating pods

3. **Example main.go** demonstrating Kubernetes deployment usage

## Dependencies

```go
require (
    k8s.io/api v0.28.0
    k8s.io/apimachinery v0.28.0
    k8s.io/client-go v0.28.0
)
```

## RBAC Requirements

Document required Kubernetes permissions:
```yaml
apiGroups: ["coordination.k8s.io"]
resources: ["leases"]
verbs: ["get", "create", "update"]
```

## Error Handling

- All backend errors bubble up through Manager
- Leadership loss (Renew failure) triggers automatic retry via TryAcquire
- Network/API errors don't panic, logged and retried
- Context cancellation cleans up gracefully

## Thread Safety

- Manager goroutine owns all backend calls
- Lease.IsLeader() uses channel select (non-blocking, thread-safe)
- No mutexes needed in public API