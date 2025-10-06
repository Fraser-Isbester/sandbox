package consensus

import (
	"context"
	"time"
)

// Backend abstracts the storage mechanism for leader election.
type Backend interface {
	// TryAcquire attempts to acquire or renew leadership.
	// leaseDuration specifies how long the lease is valid before expiring.
	// Returns true if leadership was acquired/renewed, false if another leader holds it.
	TryAcquire(ctx context.Context, identity string, leaseDuration time.Duration) (bool, error)

	// Renew extends the current leader's lease.
	// leaseDuration specifies how long the lease is valid before expiring.
	// Returns error if we're no longer the leader.
	Renew(ctx context.Context, identity string, leaseDuration time.Duration) error

	// Release explicitly gives up leadership.
	Release(ctx context.Context, identity string) error
}
