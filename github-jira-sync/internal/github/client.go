package github

import (
	"context"
	"fmt"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client
type Client struct {
	client *github.Client
	ctx    context.Context
}

// NewClient creates a new GitHub client
func NewClient(ctx context.Context, token string) *Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return &Client{
		client: github.NewClient(tc),
		ctx:    ctx,
	}
}

// Issue represents a GitHub issue with sync metadata
type Issue struct {
	Number int
	Title  string
	Body   string
	State  string
	Labels []string
	JiraKey string // Stored in issue body or as a label
}

// CreateIssue creates a new GitHub issue
func (c *Client) CreateIssue(owner, repo, title, body string, labels []string) (*Issue, error) {
	issueReq := &github.IssueRequest{
		Title:  &title,
		Body:   &body,
		Labels: &labels,
	}

	issue, _, err := c.client.Issues.Create(c.ctx, owner, repo, issueReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub issue: %w", err)
	}

	return convertGitHubIssue(issue), nil
}

// UpdateIssue updates an existing GitHub issue
func (c *Client) UpdateIssue(owner, repo string, number int, title, body *string, state *string, labels []string) (*Issue, error) {
	issueReq := &github.IssueRequest{
		Title: title,
		Body:  body,
		State: state,
	}

	if len(labels) > 0 {
		issueReq.Labels = &labels
	}

	issue, _, err := c.client.Issues.Edit(c.ctx, owner, repo, number, issueReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update GitHub issue: %w", err)
	}

	return convertGitHubIssue(issue), nil
}

// GetIssue retrieves a GitHub issue by number
func (c *Client) GetIssue(owner, repo string, number int) (*Issue, error) {
	issue, _, err := c.client.Issues.Get(c.ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("failed to get GitHub issue: %w", err)
	}

	return convertGitHubIssue(issue), nil
}

// AddComment adds a comment to a GitHub issue
func (c *Client) AddComment(owner, repo string, number int, comment string) error {
	issueComment := &github.IssueComment{
		Body: &comment,
	}

	_, _, err := c.client.Issues.CreateComment(c.ctx, owner, repo, number, issueComment)
	if err != nil {
		return fmt.Errorf("failed to add comment to GitHub issue: %w", err)
	}

	return nil
}

// CloseIssue closes a GitHub issue
func (c *Client) CloseIssue(owner, repo string, number int) error {
	state := "closed"
	_, err := c.UpdateIssue(owner, repo, number, nil, nil, &state, nil)
	return err
}

// ReopenIssue reopens a GitHub issue
func (c *Client) ReopenIssue(owner, repo string, number int) error {
	state := "open"
	_, err := c.UpdateIssue(owner, repo, number, nil, nil, &state, nil)
	return err
}

// convertGitHubIssue converts a GitHub API issue to our internal representation
func convertGitHubIssue(issue *github.Issue) *Issue {
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		if label.Name != nil {
			labels = append(labels, *label.Name)
		}
	}

	result := &Issue{
		Number: *issue.Number,
		Title:  *issue.Title,
		State:  *issue.State,
		Labels: labels,
	}

	if issue.Body != nil {
		result.Body = *issue.Body
	}

	return result
}
