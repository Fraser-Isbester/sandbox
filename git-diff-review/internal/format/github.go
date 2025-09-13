package format

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/reviewer"
)

type GitHubFormatter struct {
	repoPath string
}

func newGitHubFormatter() *GitHubFormatter {
	repoPath := os.Getenv("GITHUB_WORKSPACE")
	if repoPath == "" {
		// When running locally, we need to handle the path differently
		repoPath, _ = os.Getwd()
	}
	return &GitHubFormatter{repoPath: repoPath}
}

func (g *GitHubFormatter) Format(reviews []reviewer.ReviewResponse, writer io.Writer) error {
	for _, review := range reviews {
		// Handle path resolution
		relPath := review.File

		// If it's an absolute path, try to make it relative
		if filepath.IsAbs(review.File) {
			// Try to find the common project root
			projectPath := g.findProjectRoot(review.File)
			if projectPath != "" {
				var err error
				relPath, err = filepath.Rel(projectPath, review.File)
				if err != nil {
					return fmt.Errorf("failed to get relative path: %w", err)
				}
			}
		}

		for _, comment := range review.Comments {
			level := "notice"
			switch comment.Type {
			case "error", "issue":
				level = "error"
			case "warning":
				level = "warning"
			}

			fmt.Fprintf(writer, "::%s file=%s,line=%d,title=%s::%s\n",
				level,
				relPath,
				comment.Line,
				comment.Type,
				comment.Message,
			)

			if comment.Suggestion != "" {
				fmt.Fprintf(writer, "::notice file=%s,line=%d,title=Suggestion::```%s\n%s\n```\n",
					relPath,
					comment.Line,
					filepath.Ext(review.File)[1:],
					comment.Suggestion,
				)
			}
		}
	}
	return nil
}

// findProjectRoot attempts to find the root of the project by looking for common markers
func (g *GitHubFormatter) findProjectRoot(path string) string {
	dir := filepath.Dir(path)
	for dir != "/" && dir != "." {
		// Check for common project root indicators
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	return g.repoPath // fallback to repoPath if no root markers found
}
