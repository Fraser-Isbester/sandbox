package leasing

import (
	"context"
	"time"
)

// Lease represents a distributed lease that allows only one holder at a time.
// This enables leader election or exclusive access patterns across multiple processes.
type Lease interface {
	// Acquire attempts to acquire the lease.
	// Returns true if acquired, false if held by another process.
	// The lease will be held for the specified duration.
	Acquire(ctx context.Context, holderID string, duration time.Duration) (bool, error)

	// Renew extends the lease duration for the current holder.
	// Returns error if the lease is not held by holderID.
	Renew(ctx context.Context, holderID string, duration time.Duration) error

	// Release explicitly releases the lease.
	// Should be called when done with the lease to allow others to acquire it.
	Release(ctx context.Context, holderID string) error

	// IsHeld checks if the lease is currently held by the specified holder.
	// This is a non-blocking check of local state.
	IsHeld(holderID string) bool

	// GetHolder returns the current holder's ID, or empty string if not held.
	GetHolder(ctx context.Context) (string, error)
}

// Config provides common configuration for lease backends.
type Config struct {
	// Name is the unique name/key for this lease
	Name string

	// DefaultDuration is the default lease duration if not specified
	DefaultDuration time.Duration
}

// NewConfig creates a Config with sensible defaults.
func NewConfig(name string) Config {
	return Config{
		Name:            name,
		DefaultDuration: 15 * time.Second,
	}
}
