package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// SyncRecord represents a synced issue pair
type SyncRecord struct {
	GitHubOwner       string `json:"github_owner"`
	GitHubRepo        string `json:"github_repo"`
	GitHubIssueNumber int    `json:"github_issue_number"`
	JiraKey           string `json:"jira_key"`
	JiraProject       string `json:"jira_project"`
	LastSyncedAt      int64  `json:"last_synced_at"`
}

// Storage manages sync records
type Storage interface {
	SaveRecord(record *SyncRecord) error
	GetRecordByGitHub(owner, repo string, number int) (*SyncRecord, error)
	GetRecordByJira(key string) (*SyncRecord, error)
	DeleteRecord(record *SyncRecord) error
	ListRecords() ([]*SyncRecord, error)
}

// MemoryStorage implements in-memory storage
type MemoryStorage struct {
	mu      sync.RWMutex
	records map[string]*SyncRecord // keyed by GitHub issue or Jira key
}

// NewMemoryStorage creates a new in-memory storage
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		records: make(map[string]*SyncRecord),
	}
}

func (s *MemoryStorage) githubKey(owner, repo string, number int) string {
	return fmt.Sprintf("gh:%s/%s#%d", owner, repo, number)
}

func (s *MemoryStorage) jiraKey(key string) string {
	return fmt.Sprintf("jira:%s", key)
}

func (s *MemoryStorage) SaveRecord(record *SyncRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ghKey := s.githubKey(record.GitHubOwner, record.GitHubRepo, record.GitHubIssueNumber)
	jKey := s.jiraKey(record.JiraKey)

	s.records[ghKey] = record
	s.records[jKey] = record

	return nil
}

func (s *MemoryStorage) GetRecordByGitHub(owner, repo string, number int) (*SyncRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.githubKey(owner, repo, number)
	record, ok := s.records[key]
	if !ok {
		return nil, fmt.Errorf("record not found")
	}

	return record, nil
}

func (s *MemoryStorage) GetRecordByJira(key string) (*SyncRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jKey := s.jiraKey(key)
	record, ok := s.records[jKey]
	if !ok {
		return nil, fmt.Errorf("record not found")
	}

	return record, nil
}

func (s *MemoryStorage) DeleteRecord(record *SyncRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ghKey := s.githubKey(record.GitHubOwner, record.GitHubRepo, record.GitHubIssueNumber)
	jKey := s.jiraKey(record.JiraKey)

	delete(s.records, ghKey)
	delete(s.records, jKey)

	return nil
}

func (s *MemoryStorage) ListRecords() ([]*SyncRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	seen := make(map[string]bool)
	records := make([]*SyncRecord, 0)

	for _, record := range s.records {
		key := fmt.Sprintf("%s:%s#%d", record.GitHubOwner, record.GitHubRepo, record.GitHubIssueNumber)
		if !seen[key] {
			seen[key] = true
			records = append(records, record)
		}
	}

	return records, nil
}

// FileStorage implements file-based storage
type FileStorage struct {
	filePath string
	memory   *MemoryStorage
	mu       sync.Mutex
}

// NewFileStorage creates a new file-based storage
func NewFileStorage(filePath string) (*FileStorage, error) {
	fs := &FileStorage{
		filePath: filePath,
		memory:   NewMemoryStorage(),
	}

	// Load existing records if file exists
	if _, err := os.Stat(filePath); err == nil {
		if err := fs.load(); err != nil {
			return nil, err
		}
	}

	return fs, nil
}

func (fs *FileStorage) load() error {
	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return fmt.Errorf("failed to read storage file: %w", err)
	}

	var records []*SyncRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("failed to unmarshal records: %w", err)
	}

	for _, record := range records {
		fs.memory.SaveRecord(record)
	}

	return nil
}

func (fs *FileStorage) save() error {
	records, err := fs.memory.ListRecords()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal records: %w", err)
	}

	if err := os.WriteFile(fs.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write storage file: %w", err)
	}

	return nil
}

func (fs *FileStorage) SaveRecord(record *SyncRecord) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := fs.memory.SaveRecord(record); err != nil {
		return err
	}

	return fs.save()
}

func (fs *FileStorage) GetRecordByGitHub(owner, repo string, number int) (*SyncRecord, error) {
	return fs.memory.GetRecordByGitHub(owner, repo, number)
}

func (fs *FileStorage) GetRecordByJira(key string) (*SyncRecord, error) {
	return fs.memory.GetRecordByJira(key)
}

func (fs *FileStorage) DeleteRecord(record *SyncRecord) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if err := fs.memory.DeleteRecord(record); err != nil {
		return err
	}

	return fs.save()
}

func (fs *FileStorage) ListRecords() ([]*SyncRecord, error) {
	return fs.memory.ListRecords()
}
