package reviewer

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/prompts"
)

type ReviewComment struct {
	Type       string `json:"type"`     // "suggestion", "issue", "praise"
	Severity   string `json:"severity"` // "critical", "warning", "info"
	Line       int    `json:"line"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion,omitempty"`
}

type ReviewResponse struct {
	File     string          `json:"file"`
	Comments []ReviewComment `json:"comments"`
	Summary  string          `json:"summary"`
}

type Reviewer struct {
	llm *anthropic.LLM
}

const reviewTemplate = `Review the following code diff:

{{.diff}}

Respond only with JSON matching this format:
{
  "comments": [
    {
      "type": "suggestion|issue|praise",
      "severity": "critical|warning|info",
      "line": <integer>,
      "message": <string>,
      "suggestion": <string>
    }
  ],
  "summary": <string>
}`

func NewReviewer() (*Reviewer, error) {
	client, err := anthropic.New()
	if err != nil {
		return nil, err
	}
	return &Reviewer{llm: client}, nil
}

func shouldReviewFile(path string) bool {
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

func (r *Reviewer) ReviewDiffs(ctx context.Context, diffs []git.DiffEntry) ([]ReviewResponse, error) {
	var responses []ReviewResponse
	prompt := prompts.NewPromptTemplate(reviewTemplate, []string{"diff"})

	for _, diff := range diffs {
		if !shouldReviewFile(diff.Path) {
			continue
		}

		promptStr, err := prompt.Format(map[string]any{
			"diff": diff.Content,
		})
		if err != nil {
			return nil, err
		}

		result, err := r.llm.Call(ctx, promptStr)
		if err != nil {
			return nil, err
		}

		var response ReviewResponse
		if err := json.Unmarshal([]byte(result), &response); err != nil {
			return nil, err
		}
		response.File = diff.Path
		responses = append(responses, response)
	}
	return responses, nil
}
