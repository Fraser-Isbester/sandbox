package reviewer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/fraser-isbester/sandbox/git-diff-review/internal/git"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/anthropic"
	"github.com/tmc/langchaingo/llms/googleai"
	"github.com/tmc/langchaingo/llms/openai"
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

type Provider string

const (
	AnthropicProvider Provider = "anthropic"
	OpenAIProvider    Provider = "openai"
	GoogleAIProvider  Provider = "googleai"
)

type Reviewer struct {
	llm            llms.Model
	reviewTemplate string
}

type Config struct {
	ReviewTemplate string
	Provider       Provider
}

const defaultReviewTemplate = `Review the following code diff:

{{.diff}}

Respond only with JSON matching this format:
{
  "comments": [
    {
      "type": "suggestion|issue|praise",
      "severity": "critical|warning|info",
      "line": <integer>,
      "message": <string>,
      "suggestion": "<raw code suggestion only>"
    }
  ],
  "summary": "<one sentence summary>"
}

Rules:
1. Ensure the "suggestion" field contains only the raw code, with no additional text or context.
2. Every comment must reference exact line numbers from the diff
3. Messages must be specific to the code, never generic advice
4. Every issue must have an actionable suggestion
5. Performance comments must include expected impact
6. Security comments must explain the risk
7. No meta-commentary or summary text
8. Limit to 3-5 most important issues
9. Skip style issues unless they impact maintainability`

func NewReviewer(cfg Config) (*Reviewer, error) {
	ctx := context.Background()

	// Default to defaultReviewTemplate
	if cfg.ReviewTemplate == "" {
		cfg.ReviewTemplate = defaultReviewTemplate
	}

	// Default to Anthropic
	if cfg.Provider == "" {
		cfg.Provider = AnthropicProvider
	}

	var client llms.Model
	var err error
	switch cfg.Provider {
	case AnthropicProvider:
		client, err = anthropic.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create anthropic client: %w", err)
		}
	case OpenAIProvider:
		client, err = openai.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create openai client: %w", err)
		}
	case GoogleAIProvider:
		client, err = googleai.New(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create googleai client: %w", err)

		}
	default:
		return nil, fmt.Errorf("invalid provider: %s", cfg.Provider)
	}

	return &Reviewer{
		llm:            client,
		reviewTemplate: cfg.ReviewTemplate,
	}, nil
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
	prompt := prompts.NewPromptTemplate(r.reviewTemplate, []string{"diff"})

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
