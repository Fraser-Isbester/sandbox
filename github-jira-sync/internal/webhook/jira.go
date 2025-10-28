package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/sync"
)

// JiraHandler handles Jira webhook events
type JiraHandler struct {
	engine *sync.Engine
	secret string
}

// NewJiraHandler creates a new Jira webhook handler
func NewJiraHandler(engine *sync.Engine, secret string) *JiraHandler {
	return &JiraHandler{
		engine: engine,
		secret: secret,
	}
}

// JiraWebhookEvent represents a Jira webhook event
type JiraWebhookEvent struct {
	WebhookEvent string `json:"webhookEvent"`
	Issue        struct {
		Key    string `json:"key"`
		Fields struct {
			Summary     string `json:"summary"`
			Description string `json:"description"`
			Status      struct {
				Name string `json:"name"`
			} `json:"status"`
			Project struct {
				Key string `json:"key"`
			} `json:"project"`
		} `json:"fields"`
	} `json:"issue"`
	Changelog *struct {
		Items []struct {
			Field      string `json:"field"`
			FromString string `json:"fromString"`
			ToString   string `json:"toString"`
		} `json:"items"`
	} `json:"changelog,omitempty"`
}

// ServeHTTP handles incoming Jira webhook requests
func (h *JiraHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify signature if secret is configured
	if h.secret != "" {
		signature := r.Header.Get("X-Hub-Signature")
		if !h.verifySignature(r, signature) {
			log.Println("Invalid Jira webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var event JiraWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Failed to parse Jira event: %v", err)
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	log.Printf("Received Jira webhook: %s for issue %s", event.WebhookEvent, event.Issue.Key)

	// Handle different event types
	switch event.WebhookEvent {
	case "jira:issue_created":
		h.handleIssueCreated(event, w)
	case "jira:issue_updated":
		h.handleIssueUpdated(event, w)
	case "jira:issue_deleted":
		log.Printf("Issue deleted: %s (not syncing deletes)", event.Issue.Key)
		w.WriteHeader(http.StatusOK)
	default:
		log.Printf("Unhandled Jira event type: %s", event.WebhookEvent)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *JiraHandler) handleIssueCreated(event JiraWebhookEvent, w http.ResponseWriter) {
	issueKey := event.Issue.Key

	log.Printf("Processing Jira issue creation: %s", issueKey)

	// Check if this issue is already synced (to avoid loops)
	if h.isAlreadySynced(event.Issue.Fields.Description) {
		log.Printf("Skipping sync for already synced Jira issue %s", issueKey)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Sync to GitHub
	if err := h.engine.SyncJiraToGitHub(issueKey); err != nil {
		log.Printf("Failed to sync Jira issue to GitHub: %v", err)
		http.Error(w, "Sync failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Synced successfully"))
}

func (h *JiraHandler) handleIssueUpdated(event JiraWebhookEvent, w http.ResponseWriter) {
	issueKey := event.Issue.Key

	log.Printf("Processing Jira issue update: %s", issueKey)

	// Only sync if the issue is already linked to GitHub
	// (to avoid creating new GitHub issues on every Jira update)
	if !h.isAlreadySynced(event.Issue.Fields.Description) {
		log.Printf("Skipping sync for non-synced Jira issue %s", issueKey)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Sync to GitHub
	if err := h.engine.SyncJiraToGitHub(issueKey); err != nil {
		log.Printf("Failed to sync Jira issue to GitHub: %v", err)
		http.Error(w, "Sync failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Synced successfully"))
}

func (h *JiraHandler) isAlreadySynced(description string) bool {
	// Check if the description contains a GitHub link
	return strings.Contains(description, "Synced from GitHub:") ||
		   strings.Contains(description, "github.com")
}

func (h *JiraHandler) verifySignature(r *http.Request, signature string) bool {
	if signature == "" {
		return false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}

	// Reset body for later reading
	r.Body = io.NopCloser(strings.NewReader(string(body)))

	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(body)
	expectedMAC := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}
