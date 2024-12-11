package git

import (
	"bytes"
	"os/exec"
)

// DiffProvider interfaces with git to get repository diffs
type DiffProvider struct {
	repoPath string
}

// NewDiffProvider creates a new DiffProvider for the given repo path
func NewDiffProvider(repoPath string) *DiffProvider {
	return &DiffProvider{
		repoPath: repoPath,
	}
}

// GetCurrentDiff returns the current diff against the default branch
func (dp *DiffProvider) GetCurrentDiff() ([]byte, error) {
	cmd := exec.Command("git", "-C", dp.repoPath, "diff")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
