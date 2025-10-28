package sync

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/config"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/github"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/jira"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/storage"
)

// Engine handles bidirectional synchronization between GitHub and Jira
type Engine struct {
	config  *config.Config
	github  *github.Client
	jira    *jira.Client
	storage storage.Storage
	ctx     context.Context
}

// NewEngine creates a new sync engine
func NewEngine(ctx context.Context, cfg *config.Config, gh *github.Client, jr *jira.Client, store storage.Storage) *Engine {
	return &Engine{
		config:  cfg,
		github:  gh,
		jira:    jr,
		storage: store,
		ctx:     ctx,
	}
}

// SyncJiraToGitHub syncs a Jira issue to GitHub
func (e *Engine) SyncJiraToGitHub(jiraKey string) error {
	log.Printf("Syncing Jira issue %s to GitHub", jiraKey)

	// Get Jira issue
	jiraIssue, err := e.jira.GetIssue(jiraKey)
	if err != nil {
		return fmt.Errorf("failed to get Jira issue: %w", err)
	}

	// Find mapping for this Jira project
	mapping := e.config.GetMappingByJiraProject(jiraIssue.Project)
	if mapping == nil {
		return fmt.Errorf("no mapping found for Jira project: %s", jiraIssue.Project)
	}

	// Check if already synced
	record, err := e.storage.GetRecordByJira(jiraKey)
	if err == nil {
		// Already synced, update GitHub issue
		return e.updateGitHubFromJira(record, jiraIssue, mapping)
	}

	// Create new GitHub issue
	return e.createGitHubFromJira(jiraIssue, mapping)
}

// SyncGitHubToJira syncs a GitHub issue to Jira
func (e *Engine) SyncGitHubToJira(owner, repo string, issueNumber int) error {
	log.Printf("Syncing GitHub issue %s/%s#%d to Jira", owner, repo, issueNumber)

	// Get GitHub issue
	ghIssue, err := e.github.GetIssue(owner, repo, issueNumber)
	if err != nil {
		return fmt.Errorf("failed to get GitHub issue: %w", err)
	}

	// Find mapping for this GitHub repo
	mapping := e.config.GetMappingByGitHubRepo(owner, repo)
	if mapping == nil {
		return fmt.Errorf("no mapping found for GitHub repo: %s/%s", owner, repo)
	}

	// Check if already synced
	record, err := e.storage.GetRecordByGitHub(owner, repo, issueNumber)
	if err == nil {
		// Already synced, update Jira issue
		return e.updateJiraFromGitHub(record, ghIssue, mapping)
	}

	// Create new Jira issue
	return e.createJiraFromGitHub(ghIssue, owner, repo, mapping)
}

// createGitHubFromJira creates a new GitHub issue from a Jira issue
func (e *Engine) createGitHubFromJira(jiraIssue *jira.Issue, mapping *config.ProjectMapping) error {
	// Prepare GitHub issue body with Jira link
	body := fmt.Sprintf("**Synced from Jira: [%s]**\n\n%s\n\n---\n*This issue is synced with Jira ticket %s*",
		jiraIssue.Key, jiraIssue.Description, jiraIssue.Key)

	// Add sync labels
	labels := append(mapping.SyncLabels, "jira-sync")

	// Create GitHub issue
	ghIssue, err := e.github.CreateIssue(mapping.GitHubOwner, mapping.GitHubRepo, jiraIssue.Summary, body, labels)
	if err != nil {
		return fmt.Errorf("failed to create GitHub issue: %w", err)
	}

	// Save sync record
	record := &storage.SyncRecord{
		GitHubOwner:       mapping.GitHubOwner,
		GitHubRepo:        mapping.GitHubRepo,
		GitHubIssueNumber: ghIssue.Number,
		JiraKey:           jiraIssue.Key,
		JiraProject:       jiraIssue.Project,
		LastSyncedAt:      time.Now().Unix(),
	}

	if err := e.storage.SaveRecord(record); err != nil {
		return fmt.Errorf("failed to save sync record: %w", err)
	}

	// Add comment to Jira with GitHub link
	githubURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", mapping.GitHubOwner, mapping.GitHubRepo, ghIssue.Number)
	comment := fmt.Sprintf("Synced to GitHub: %s", githubURL)
	if err := e.jira.AddComment(jiraIssue.Key, comment); err != nil {
		log.Printf("Warning: failed to add comment to Jira: %v", err)
	}

	log.Printf("Created GitHub issue #%d from Jira %s", ghIssue.Number, jiraIssue.Key)
	return nil
}

// createJiraFromGitHub creates a new Jira issue from a GitHub issue
func (e *Engine) createJiraFromGitHub(ghIssue *github.Issue, owner, repo string, mapping *config.ProjectMapping) error {
	// Prepare Jira issue description with GitHub link
	githubURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", owner, repo, ghIssue.Number)
	description := fmt.Sprintf("Synced from GitHub: %s\n\n%s\n\n---\nThis issue is synced with GitHub issue #%d",
		githubURL, ghIssue.Body, ghIssue.Number)

	// Create Jira issue (default to "Task" type)
	jiraIssue, err := e.jira.CreateIssue(mapping.JiraProject, ghIssue.Title, description, "Task")
	if err != nil {
		return fmt.Errorf("failed to create Jira issue: %w", err)
	}

	// Save sync record
	record := &storage.SyncRecord{
		GitHubOwner:       owner,
		GitHubRepo:        repo,
		GitHubIssueNumber: ghIssue.Number,
		JiraKey:           jiraIssue.Key,
		JiraProject:       jiraIssue.Project,
		LastSyncedAt:      time.Now().Unix(),
	}

	if err := e.storage.SaveRecord(record); err != nil {
		return fmt.Errorf("failed to save sync record: %w", err)
	}

	// Add comment to GitHub with Jira link
	comment := fmt.Sprintf("Synced to Jira: %s", jiraIssue.Key)
	if err := e.github.AddComment(owner, repo, ghIssue.Number, comment); err != nil {
		log.Printf("Warning: failed to add comment to GitHub: %v", err)
	}

	log.Printf("Created Jira issue %s from GitHub #%d", jiraIssue.Key, ghIssue.Number)
	return nil
}

// updateGitHubFromJira updates an existing GitHub issue from Jira
func (e *Engine) updateGitHubFromJira(record *storage.SyncRecord, jiraIssue *jira.Issue, mapping *config.ProjectMapping) error {
	log.Printf("Updating GitHub issue #%d from Jira %s", record.GitHubIssueNumber, jiraIssue.Key)

	// Get current GitHub issue
	ghIssue, err := e.github.GetIssue(record.GitHubOwner, record.GitHubRepo, record.GitHubIssueNumber)
	if err != nil {
		return fmt.Errorf("failed to get GitHub issue: %w", err)
	}

	// Prepare updated body
	body := fmt.Sprintf("**Synced from Jira: [%s]**\n\n%s\n\n---\n*This issue is synced with Jira ticket %s*",
		jiraIssue.Key, jiraIssue.Description, jiraIssue.Key)

	// Determine state based on Jira status
	var state *string
	if isJiraStatusClosed(jiraIssue.Status) && ghIssue.State == "open" {
		closed := "closed"
		state = &closed
	} else if !isJiraStatusClosed(jiraIssue.Status) && ghIssue.State == "closed" {
		open := "open"
		state = &open
	}

	// Update GitHub issue
	_, err = e.github.UpdateIssue(record.GitHubOwner, record.GitHubRepo, record.GitHubIssueNumber,
		&jiraIssue.Summary, &body, state, nil)
	if err != nil {
		return fmt.Errorf("failed to update GitHub issue: %w", err)
	}

	// Update sync record
	record.LastSyncedAt = time.Now().Unix()
	if err := e.storage.SaveRecord(record); err != nil {
		return fmt.Errorf("failed to update sync record: %w", err)
	}

	log.Printf("Updated GitHub issue #%d", record.GitHubIssueNumber)
	return nil
}

// updateJiraFromGitHub updates an existing Jira issue from GitHub
func (e *Engine) updateJiraFromGitHub(record *storage.SyncRecord, ghIssue *github.Issue, mapping *config.ProjectMapping) error {
	log.Printf("Updating Jira issue %s from GitHub #%d", record.JiraKey, ghIssue.Number)

	// Get current Jira issue
	jiraIssue, err := e.jira.GetIssue(record.JiraKey)
	if err != nil {
		return fmt.Errorf("failed to get Jira issue: %w", err)
	}

	// Prepare updated description
	githubURL := fmt.Sprintf("https://github.com/%s/%s/issues/%d", record.GitHubOwner, record.GitHubRepo, ghIssue.Number)
	description := fmt.Sprintf("Synced from GitHub: %s\n\n%s\n\n---\nThis issue is synced with GitHub issue #%d",
		githubURL, ghIssue.Body, ghIssue.Number)

	// Update Jira issue
	_, err = e.jira.UpdateIssue(record.JiraKey, ghIssue.Title, description)
	if err != nil {
		return fmt.Errorf("failed to update Jira issue: %w", err)
	}

	// Handle state transitions
	if ghIssue.State == "closed" && !isJiraStatusClosed(jiraIssue.Status) {
		// Try to close Jira issue (transition names vary by workflow)
		if err := e.jira.TransitionIssue(record.JiraKey, "Done"); err != nil {
			log.Printf("Warning: failed to close Jira issue: %v", err)
		}
	}

	// Update sync record
	record.LastSyncedAt = time.Now().Unix()
	if err := e.storage.SaveRecord(record); err != nil {
		return fmt.Errorf("failed to update sync record: %w", err)
	}

	log.Printf("Updated Jira issue %s", record.JiraKey)
	return nil
}

// isJiraStatusClosed checks if a Jira status indicates the issue is closed
func isJiraStatusClosed(status string) bool {
	closedStatuses := []string{"Done", "Closed", "Resolved", "Complete"}
	for _, s := range closedStatuses {
		if strings.EqualFold(status, s) {
			return true
		}
	}
	return false
}
