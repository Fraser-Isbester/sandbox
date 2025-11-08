package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
	"github.com/redis/go-redis/v9"
)

var (
	ErrLeaseHeldByAnother = errors.New("lease is held by another process")
	ErrNotLeaseHolder     = errors.New("not the current lease holder")
)

// RedisLease implements a Redis-based distributed lease using SET with NX and EX options.
// This provides atomic lease acquisition across multiple processes/nodes.
type RedisLease struct {
	client *redis.Client
	config leasing.Config
	key    string
}

// New creates a new Redis-based lease.
// The client should already be configured and connected to Redis.
func New(client *redis.Client, config leasing.Config) (*RedisLease, error) {
	if client == nil {
		return nil, errors.New("redis client is required")
	}
	if config.Name == "" {
		return nil, errors.New("lease name is required")
	}

	key := fmt.Sprintf("lease:%s", config.Name)

	return &RedisLease{
		client: client,
		config: config,
		key:    key,
	}, nil
}

// NewWithOptions creates a Redis client and a new Redis-based lease.
func NewWithOptions(addr string, config leasing.Config, opts *redis.Options) (*RedisLease, error) {
	if opts == nil {
		opts = &redis.Options{
			Addr: addr,
		}
	} else if opts.Addr == "" {
		opts.Addr = addr
	}

	client := redis.NewClient(opts)
	return New(client, config)
}

// Acquire attempts to acquire the lease using Redis SET with NX (set if not exists).
func (r *RedisLease) Acquire(ctx context.Context, holderID string, duration time.Duration) (bool, error) {
	// Try to set the key only if it doesn't exist (NX)
	// EX sets the expiration time
	result, err := r.client.SetNX(ctx, r.key, holderID, duration).Result()
	if err != nil {
		return false, fmt.Errorf("redis set failed: %w", err)
	}

	// If result is true, we acquired the lease
	// If false, someone else holds it
	return result, nil
}

// Renew extends the lease duration for the current holder.
// Uses Lua script to atomically check holder and update expiration.
func (r *RedisLease) Renew(ctx context.Context, holderID string, duration time.Duration) error {
	// Lua script to atomically check and renew
	// KEYS[1] = lease key
	// ARGV[1] = holder ID
	// ARGV[2] = duration in milliseconds
	script := `
		local current = redis.call('GET', KEYS[1])
		if current == ARGV[1] then
			redis.call('PEXPIRE', KEYS[1], ARGV[2])
			return 1
		else
			return 0
		end
	`

	result, err := r.client.Eval(ctx, script, []string{r.key}, holderID, duration.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("redis eval failed: %w", err)
	}

	if result == 0 {
		return ErrNotLeaseHolder
	}

	return nil
}

// Release releases the lease only if held by the specified holder.
// Uses Lua script for atomic check-and-delete.
func (r *RedisLease) Release(ctx context.Context, holderID string) error {
	// Lua script to atomically check and delete
	// KEYS[1] = lease key
	// ARGV[1] = holder ID
	script := `
		local current = redis.call('GET', KEYS[1])
		if current == ARGV[1] then
			redis.call('DEL', KEYS[1])
			return 1
		else
			return 0
		end
	`

	result, err := r.client.Eval(ctx, script, []string{r.key}, holderID).Int()
	if err != nil {
		return fmt.Errorf("redis eval failed: %w", err)
	}

	if result == 0 {
		return ErrNotLeaseHolder
	}

	return nil
}

// IsHeld checks if the lease is held by the specified holder.
// This performs a Redis GET to check the current value.
func (r *RedisLease) IsHeld(holderID string) bool {
	ctx := context.Background()
	val, err := r.client.Get(ctx, r.key).Result()
	if err != nil {
		return false
	}
	return val == holderID
}

// GetHolder returns the current holder's ID.
func (r *RedisLease) GetHolder(ctx context.Context) (string, error) {
	val, err := r.client.Get(ctx, r.key).Result()
	if err == redis.Nil {
		return "", nil // No holder
	}
	if err != nil {
		return "", fmt.Errorf("redis get failed: %w", err)
	}
	return val, nil
}

// Close closes the Redis client connection.
func (r *RedisLease) Close() error {
	return r.client.Close()
}
