package consensus

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Config defines the configuration for leader election.
type Config struct {
	Identity      string        // Unique identifier for this instance (e.g., POD_NAME)
	LeaseDuration time.Duration // How long a lease is valid before expiring
	RenewInterval time.Duration // How often the leader renews its lease
	RetryInterval time.Duration // How often non-leaders retry acquiring leadership
}

// NewConfig creates a Config with sensible defaults.
// Defaults: LeaseDuration=15s, RenewInterval=5s, RetryInterval=2s
func NewConfig(identity string) Config {
	return Config{
		Identity:      identity,
		LeaseDuration: 5 * time.Second,
		RenewInterval: 3 * time.Second,
		RetryInterval: 2 * time.Second,
	}
}

// Manager manages leader election using a pluggable backend.
type Manager struct {
	backend Backend
	config  Config

	mu            sync.Mutex
	lease         *Lease
	cancel        context.CancelFunc
	stopOnce      sync.Once
	renewFailures int
}

// NewManager creates a new leader election manager.
func NewManager(backend Backend, config Config) *Manager {
	return &Manager{
		backend: backend,
		config:  config,
	}
}

// Start begins the leader election process.
// Returns immediately with a Lease for checking leadership status.
func (m *Manager) Start(ctx context.Context) *Lease {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	lease := &Lease{
		isLeader: atomic.Bool{},
		leaderCh: make(chan struct{}),
	}
	lease.isLeader.Store(false)

	m.mu.Lock()
	m.lease = lease
	m.mu.Unlock()

	go m.run(ctx)

	return lease
}

// Stop gracefully stops leader election and releases leadership if held.
func (m *Manager) Stop() error {
	var err error
	m.stopOnce.Do(func() {
		if m.cancel != nil {
			m.cancel()
		}
	})
	return err
}

// run is the main election loop that runs in a goroutine.
func (m *Manager) run(ctx context.Context) {
	ticker := time.NewTicker(m.config.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Release leadership if we hold it
			if m.lease.IsLeader() {
				releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = m.backend.Release(releaseCtx, m.config.Identity)
				cancel()
			}
			return

		case <-ticker.C:
			m.tick(ctx)
		}
	}
}

// tick handles one iteration of the election loop.
func (m *Manager) tick(ctx context.Context) {
	if m.lease.IsLeader() {
		// We're the leader - try to renew
		err := m.backend.Renew(ctx, m.config.Identity, m.config.LeaseDuration)
		if err != nil {
			m.renewFailures++
			// Allow transient failures, but demote after consecutive failures
			if m.renewFailures >= 2 {
				m.loseLeadership()
			}
		} else {
			m.renewFailures = 0
		}
	} else {
		// We're not the leader - try to acquire
		acquired, err := m.backend.TryAcquire(ctx, m.config.Identity, m.config.LeaseDuration)
		if err == nil && acquired {
			m.gainLeadership()
		}
	}

	// Adjust ticker interval based on leadership state
	m.adjustTicker()
}

// gainLeadership transitions to leader state.
func (m *Manager) gainLeadership() {
	if !m.lease.isLeader.Swap(true) {
		// We just became leader
		close(m.lease.leaderCh)
		m.renewFailures = 0
	}
}

// loseLeadership transitions to non-leader state.
func (m *Manager) loseLeadership() {
	if m.lease.isLeader.Swap(false) {
		// We just lost leadership - recreate the channel
		m.lease.mu.Lock()
		m.lease.leaderCh = make(chan struct{})
		m.lease.mu.Unlock()
		m.renewFailures = 0
	}
}

// adjustTicker adjusts the ticker interval based on leadership state.
func (m *Manager) adjustTicker() {
	// This is a simplified version - in practice, we'd need to recreate the ticker
	// with the appropriate interval. For POC, we'll use RetryInterval as base.
}

// Lease represents a lease on leadership that can be queried.
type Lease struct {
	isLeader atomic.Bool
	mu       sync.Mutex
	leaderCh chan struct{}
}

// IsLeader returns true if this instance is currently the leader.
// Non-blocking, safe to call in tight loops.
func (l *Lease) IsLeader() bool {
	return l.isLeader.Load()
}

// WaitForLeadership blocks until this instance becomes leader or context cancels.
func (l *Lease) WaitForLeadership(ctx context.Context) error {
	// If already leader, return immediately
	if l.IsLeader() {
		return nil
	}

	// Wait for leadership signal
	l.mu.Lock()
	ch := l.leaderCh
	l.mu.Unlock()

	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
