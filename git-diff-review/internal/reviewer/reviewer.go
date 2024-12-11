package reviewer

import (
	"context"
	"regexp"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/fraser-isbester/sandbox/git-diff-review/internal/llm"
)

// Review represents a code review result
type Review struct {
	FilePath   string
	LineNumber int
	Suggestion string
	Severity   string
}

type Reviewer struct {
	llmClient *llm.Client
}

func NewReviewer() (*Reviewer, error) {
	client, err := llm.NewClient()
	if err != nil {
		return nil, err
	}
	return &Reviewer{llmClient: client}, nil
}

func shouldReviewFile(path string) bool {
	// Manually skip some files
	skipPatterns := []string{
		"go.mod$",
		"go.sum$",
		"package-lock.json$",
		"yarn.lock$",
		".gitignore$",
	}

	for _, pattern := range skipPatterns {
		if matched, _ := regexp.MatchString(pattern, path); matched {
			return false
		}
	}
	return true
}

func (r *Reviewer) ReviewDiffs(ctx context.Context, diffs []git.DiffEntry) ([]Review, error) {
	var reviews []Review
	for _, diff := range diffs {
		if !shouldReviewFile(diff.Path) {
			continue
		}
		feedback, err := r.llmClient.Review(ctx, diff.Content)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, Review{
			FilePath:   diff.Path,
			LineNumber: diff.LineNum,
			Suggestion: feedback,
			Severity:   "info",
		})
	}
	return reviews, nil
}
