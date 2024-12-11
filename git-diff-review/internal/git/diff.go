package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
)

// DiffEntry represents a single change in a diff
type DiffEntry struct {
	Path    string
	Content string
	LineNum int
	Type    ChangeType
}

// ChangeType represents the type of change in a diff
type ChangeType int

const (
	Add ChangeType = iota
	Delete
	Modify
)

// DiffProvider handles git repository operations
type DiffProvider struct {
	repo *git.Repository
}

func findGitRoot(path string) (string, error) {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// NewDiffProvider creates a DiffProvider for the given path
func NewDiffProvider(path string) (*DiffProvider, error) {
	rootPath, err := findGitRoot(path)
	if err != nil {
		return nil, err
	}

	r, err := git.PlainOpen(rootPath)
	if err != nil {
		return nil, err
	}
	return &DiffProvider{repo: r}, nil
}

// GetCurrentDiff returns changes between HEAD and working directory
func (dp *DiffProvider) GetCurrentDiff() ([]DiffEntry, error) {
	w, err := dp.repo.Worktree()
	if err != nil {
		return nil, err
	}

	status, err := w.Status()
	if err != nil {
		return nil, err
	}

	var entries []DiffEntry
	for path, fileStatus := range status {
		if fileStatus.Staging == git.Untracked {
			continue
		}

		entry := DiffEntry{
			Path: path,
			Type: determineChangeType(fileStatus.Staging),
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func determineChangeType(status git.StatusCode) ChangeType {
	switch status {
	case git.Added:
		return Add
	case git.Deleted:
		return Delete
	default:
		return Modify
	}
}
