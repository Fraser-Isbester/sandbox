package llm

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
)

type Client struct {
	client *anthropic.Client
}

func NewClient() (*Client, error) {

	client := anthropic.NewClient()
	return &Client{client: client}, nil
}

func (c *Client) Review(ctx context.Context, diff string) (string, error) {
	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.F(anthropic.ModelClaude_3_5_Sonnet_20240620),
		MaxTokens: anthropic.F(int64(1024)),
		Messages: anthropic.F([]anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(
				fmt.Sprintf("Review this code diff:\n```\n%s\n```", diff),
			)),
		}),
		System: anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(reviewTemplate),
		}),
	})
	if err != nil {
		return "", fmt.Errorf("llm review failed: %w", err)
	}

	return msg.Content[0].Text, nil
}

// todo: acutally template this
const reviewTemplate = `You are performing an automated code review. Examine the following diff:

{diff}

Provide a JSON response with exact line numbers and specific, non-generic feedback. Format:

{
  "comments": [
    {
      "type": "issue"|"suggestion"|"security"|"performance"|"style",
      "severity": "critical"|"warning"|"info",
      "line": <exact_line_number>,
      "message": "<one sentence describing the specific issue>",
      "suggestion": "<specific code or change recommendation>"
    }
  ]
}

Rules:
1. Every comment must reference exact line numbers from the diff
2. Messages must be specific to the code, never generic advice
3. Every issue must have an actionable suggestion
4. Performance comments must include expected impact
5. Security comments must explain the risk
6. No meta-commentary or summary text
7. Limit to 3-5 most important issues
8. Skip style issues unless they impact maintainability`
