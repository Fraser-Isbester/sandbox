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
	repo     *git.Repository
	repoPath string
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
	return &DiffProvider{
		repo:     r,
		repoPath: rootPath,
	}, nil
}

// GetCurrentDiff returns changes between HEAD and working directory
func (dp *DiffProvider) GetCurrentDiff() ([]DiffEntry, error) {
	// Get raw diff output
	cmd := exec.Command("git", "diff")
	cmd.Dir = dp.repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get diff: %w", err)
	}

	// Parse unified diff format
	entries := []DiffEntry{}
	files := strings.Split(string(out), "diff --git")

	for _, file := range files[1:] { // Skip first empty element
		lines := strings.Split(file, "\n")
		if len(lines) < 3 {
			continue
		}

		// Extract filename from diff header
		// Example: "a/path/file.go b/path/file.go"
		pathLine := strings.Fields(lines[0])
		if len(pathLine) < 2 {
			continue
		}
		path := strings.TrimPrefix(pathLine[1], "b/")

		// Collect actual diff content
		content := strings.Join(lines[3:], "\n")

		entries = append(entries, DiffEntry{
			Path:    path,
			Content: content,
			Type:    Modify, // For now, simplify to always Modify
		})
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
