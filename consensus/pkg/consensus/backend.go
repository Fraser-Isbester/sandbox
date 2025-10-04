package consensus

import "context"

// Backend abstracts the storage mechanism for leader election.
type Backend interface {
	// TryAcquire attempts to acquire or renew leadership.
	// Returns true if leadership was acquired/renewed, false if another leader holds it.
	TryAcquire(ctx context.Context, identity string) (bool, error)

	// Renew extends the current leader's lease.
	// Returns error if we're no longer the leader.
	Renew(ctx context.Context, identity string) error

	// Release explicitly gives up leadership.
	Release(ctx context.Context, identity string) error
}
