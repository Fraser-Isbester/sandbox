package file

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"time"
)

// leaseData represents the JSON structure stored in the lease file.
type leaseData struct {
	Holder    string    `json:"holder"`
	RenewTime time.Time `json:"renewTime"`
}

// Backend implements consensus.Backend using a file-based lock.
type Backend struct {
	path string
	ttl  time.Duration
}

// NewBackend creates a new file-based backend.
func NewBackend(path string, ttl time.Duration) *Backend {
	return &Backend{
		path: path,
		ttl:  ttl,
	}
}

// TryAcquire attempts to acquire or renew leadership.
func (b *Backend) TryAcquire(ctx context.Context, identity string) (bool, error) {
	return b.withLock(func(file *os.File) (bool, error) {
		data, err := b.readLease(file)
		if err != nil {
			return false, err
		}

		now := time.Now()

		// If we're already the holder, renew
		if data.Holder == identity {
			data.RenewTime = now
			if err := b.writeLease(file, data); err != nil {
				return false, err
			}
			return true, nil
		}

		// If no holder or lease expired, acquire
		if data.Holder == "" || now.Sub(data.RenewTime) > b.ttl {
			data.Holder = identity
			data.RenewTime = now
			if err := b.writeLease(file, data); err != nil {
				return false, err
			}
			return true, nil
		}

		// Someone else holds a valid lease
		return false, nil
	})
}

// Renew extends the current leader's lease.
func (b *Backend) Renew(ctx context.Context, identity string) error {
	acquired, err := b.withLock(func(file *os.File) (bool, error) {
		data, err := b.readLease(file)
		if err != nil {
			return false, err
		}

		// Verify we're the holder
		if data.Holder != identity {
			return false, fmt.Errorf("not the lease holder")
		}

		// Update renewal time
		data.RenewTime = time.Now()
		if err := b.writeLease(file, data); err != nil {
			return false, err
		}

		return true, nil
	})

	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("failed to renew: not the holder")
	}

	return nil
}

// Release explicitly gives up leadership.
func (b *Backend) Release(ctx context.Context, identity string) error {
	_, err := b.withLock(func(file *os.File) (bool, error) {
		data, err := b.readLease(file)
		if err != nil {
			return false, err
		}

		// Only release if we're the holder
		if data.Holder == identity {
			data.Holder = ""
			if err := b.writeLease(file, data); err != nil {
				return false, err
			}
		}

		return true, nil
	})

	return err
}

// withLock executes a function while holding an exclusive file lock.
func (b *Backend) withLock(fn func(*os.File) (bool, error)) (bool, error) {
	// Open or create the file
	file, err := os.OpenFile(b.path, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return false, fmt.Errorf("failed to open lease file: %w", err)
	}
	defer file.Close()

	// Acquire exclusive lock
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return false, fmt.Errorf("failed to acquire file lock: %w", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	return fn(file)
}

// readLease reads the lease data from the file.
func (b *Backend) readLease(file *os.File) (*leaseData, error) {
	// Seek to beginning
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to seek: %w", err)
	}

	// Check if file is empty
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if stat.Size() == 0 {
		// Empty file - return empty lease
		return &leaseData{}, nil
	}

	// Read and parse JSON
	var data leaseData
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode lease: %w", err)
	}

	return &data, nil
}

// writeLease writes the lease data to the file.
func (b *Backend) writeLease(file *os.File, data *leaseData) error {
	// Seek to beginning
	if _, err := file.Seek(0, 0); err != nil {
		return fmt.Errorf("failed to seek: %w", err)
	}

	// Truncate file
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("failed to truncate: %w", err)
	}

	// Write JSON
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("failed to encode lease: %w", err)
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	return nil
}
