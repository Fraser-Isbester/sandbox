package jira

import (
	"context"
	"fmt"
	"strings"

	jira "github.com/andygrunwald/go-jira"
)

// Client wraps the Jira API client
type Client struct {
	client *jira.Client
	ctx    context.Context
}

// NewClient creates a new Jira client
func NewClient(ctx context.Context, url, username, apiToken string) (*Client, error) {
	tp := jira.BasicAuthTransport{
		Username: username,
		Password: apiToken,
	}

	client, err := jira.NewClient(tp.Client(), url)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira client: %w", err)
	}

	return &Client{
		client: client,
		ctx:    ctx,
	}, nil
}

// Issue represents a Jira issue with sync metadata
type Issue struct {
	Key         string
	Summary     string
	Description string
	Status      string
	IssueType   string
	Project     string
	GitHubIssueNumber int    // Stored in custom field or description
	GitHubRepo        string // Stored in custom field or description
}

// CreateIssue creates a new Jira issue
func (c *Client) CreateIssue(project, summary, description, issueType string) (*Issue, error) {
	i := jira.Issue{
		Fields: &jira.IssueFields{
			Project: jira.Project{
				Key: project,
			},
			Summary:     summary,
			Description: description,
			Type: jira.IssueType{
				Name: issueType,
			},
		},
	}

	issue, _, err := c.client.Issue.Create(&i)
	if err != nil {
		return nil, fmt.Errorf("failed to create Jira issue: %w", err)
	}

	return convertJiraIssue(issue), nil
}

// UpdateIssue updates an existing Jira issue
func (c *Client) UpdateIssue(key, summary, description string) (*Issue, error) {
	issue, _, err := c.client.Issue.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira issue: %w", err)
	}

	if summary != "" {
		issue.Fields.Summary = summary
	}
	if description != "" {
		issue.Fields.Description = description
	}

	updatedIssue, _, err := c.client.Issue.Update(issue)
	if err != nil {
		return nil, fmt.Errorf("failed to update Jira issue: %w", err)
	}

	return convertJiraIssue(updatedIssue), nil
}

// GetIssue retrieves a Jira issue by key
func (c *Client) GetIssue(key string) (*Issue, error) {
	issue, _, err := c.client.Issue.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get Jira issue: %w", err)
	}

	return convertJiraIssue(issue), nil
}

// AddComment adds a comment to a Jira issue
func (c *Client) AddComment(key, comment string) error {
	jiraComment := &jira.Comment{
		Body: comment,
	}

	_, _, err := c.client.Issue.AddComment(key, jiraComment)
	if err != nil {
		return fmt.Errorf("failed to add comment to Jira issue: %w", err)
	}

	return nil
}

// TransitionIssue transitions a Jira issue to a new status
func (c *Client) TransitionIssue(key, transitionName string) error {
	// Get available transitions
	transitions, _, err := c.client.Issue.GetTransitions(key)
	if err != nil {
		return fmt.Errorf("failed to get transitions: %w", err)
	}

	// Find the transition ID by name
	var transitionID string
	for _, t := range transitions {
		if strings.EqualFold(t.Name, transitionName) {
			transitionID = t.ID
			break
		}
	}

	if transitionID == "" {
		return fmt.Errorf("transition '%s' not found", transitionName)
	}

	// Execute transition
	_, err = c.client.Issue.DoTransition(key, transitionID)
	if err != nil {
		return fmt.Errorf("failed to transition issue: %w", err)
	}

	return nil
}

// SearchIssuesByLabel searches for Jira issues with a specific label
func (c *Client) SearchIssuesByLabel(project, label string) ([]*Issue, error) {
	jql := fmt.Sprintf("project = %s AND labels = %s", project, label)
	issues, _, err := c.client.Issue.Search(jql, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search Jira issues: %w", err)
	}

	result := make([]*Issue, 0, len(issues))
	for i := range issues {
		result = append(result, convertJiraIssue(&issues[i]))
	}

	return result, nil
}

// convertJiraIssue converts a Jira API issue to our internal representation
func convertJiraIssue(issue *jira.Issue) *Issue {
	result := &Issue{
		Key:         issue.Key,
		Summary:     issue.Fields.Summary,
		Description: issue.Fields.Description,
		Project:     issue.Fields.Project.Key,
	}

	if issue.Fields.Status != nil {
		result.Status = issue.Fields.Status.Name
	}

	if issue.Fields.Type.Name != "" {
		result.IssueType = issue.Fields.Type.Name
	}

	return result
}
