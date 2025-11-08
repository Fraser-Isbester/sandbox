package file

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Fraser-Isbester/sandbox/leasing-go/pkg/leasing"
)

var (
	ErrLeaseHeldByAnother = errors.New("lease is held by another process")
	ErrNotLeaseHolder     = errors.New("not the current lease holder")
)

// leaseData is the structure stored in the lease file.
type leaseData struct {
	HolderID   string    `json:"holder_id"`
	ExpiresAt  time.Time `json:"expires_at"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// FileLease implements a file-based lease using file locking.
// Suitable for single-node scenarios or shared filesystems.
type FileLease struct {
	config       leasing.Config
	filePath     string
	mu           sync.RWMutex
	currentData  *leaseData
	lastModified time.Time
}

// New creates a new file-based lease.
// The lease file will be created in the specified directory.
func New(config leasing.Config, directory string) (*FileLease, error) {
	if config.Name == "" {
		return nil, errors.New("lease name is required")
	}

	// Ensure directory exists
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	filePath := filepath.Join(directory, fmt.Sprintf("%s.lease", config.Name))

	return &FileLease{
		config:   config,
		filePath: filePath,
	}, nil
}

// Acquire attempts to acquire the lease.
func (f *FileLease) Acquire(ctx context.Context, holderID string, duration time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Read current lease state
	current, err := f.readLeaseFile()
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to read lease file: %w", err)
	}

	now := time.Now()

	// Check if lease exists and is still valid
	if current != nil && now.Before(current.ExpiresAt) {
		if current.HolderID != holderID {
			return false, nil // Held by another process
		}
		// We already hold it, extend it
		return true, f.writeLeaseFile(holderID, duration)
	}

	// Lease is available or expired, acquire it
	if err := f.writeLeaseFile(holderID, duration); err != nil {
		return false, err
	}

	f.currentData = &leaseData{
		HolderID:   holderID,
		ExpiresAt:  now.Add(duration),
		AcquiredAt: now,
	}

	return true, nil
}

// Renew extends the lease duration.
func (f *FileLease) Renew(ctx context.Context, holderID string, duration time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Read current lease state
	current, err := f.readLeaseFile()
	if err != nil {
		if os.IsNotExist(err) {
			return ErrNotLeaseHolder
		}
		return fmt.Errorf("failed to read lease file: %w", err)
	}

	now := time.Now()

	// Verify we're the holder
	if current.HolderID != holderID {
		return ErrNotLeaseHolder
	}

	// Check if expired
	if now.After(current.ExpiresAt) {
		return ErrNotLeaseHolder
	}

	// Renew the lease
	return f.writeLeaseFile(holderID, duration)
}

// Release releases the lease.
func (f *FileLease) Release(ctx context.Context, holderID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Read current lease state
	current, err := f.readLeaseFile()
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already released
		}
		return fmt.Errorf("failed to read lease file: %w", err)
	}

	// Verify we're the holder
	if current.HolderID != holderID {
		return ErrNotLeaseHolder
	}

	// Remove the lease file
	if err := os.Remove(f.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lease file: %w", err)
	}

	f.currentData = nil
	return nil
}

// IsHeld checks if the lease is held by the specified holder.
func (f *FileLease) IsHeld(holderID string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if f.currentData == nil {
		return false
	}

	return f.currentData.HolderID == holderID && time.Now().Before(f.currentData.ExpiresAt)
}

// GetHolder returns the current holder's ID.
func (f *FileLease) GetHolder(ctx context.Context) (string, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	current, err := f.readLeaseFile()
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to read lease file: %w", err)
	}

	// Check if expired
	if time.Now().After(current.ExpiresAt) {
		return "", nil
	}

	return current.HolderID, nil
}

// readLeaseFile reads and parses the lease file.
func (f *FileLease) readLeaseFile() (*leaseData, error) {
	data, err := os.ReadFile(f.filePath)
	if err != nil {
		return nil, err
	}

	var lease leaseData
	if err := json.Unmarshal(data, &lease); err != nil {
		return nil, fmt.Errorf("failed to parse lease file: %w", err)
	}

	return &lease, nil
}

// writeLeaseFile writes the lease data to the file.
func (f *FileLease) writeLeaseFile(holderID string, duration time.Duration) error {
	now := time.Now()
	data := leaseData{
		HolderID:   holderID,
		ExpiresAt:  now.Add(duration),
		AcquiredAt: now,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal lease data: %w", err)
	}

	// Write atomically by writing to temp file and renaming
	tmpPath := f.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write lease file: %w", err)
	}

	if err := os.Rename(tmpPath, f.filePath); err != nil {
		os.Remove(tmpPath) // Clean up temp file
		return fmt.Errorf("failed to rename lease file: %w", err)
	}

	f.currentData = &data
	f.lastModified = now

	return nil
}
