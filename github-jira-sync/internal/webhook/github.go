package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/sync"
)

// GitHubHandler handles GitHub webhook events
type GitHubHandler struct {
	engine *sync.Engine
	secret string
}

// NewGitHubHandler creates a new GitHub webhook handler
func NewGitHubHandler(engine *sync.Engine, secret string) *GitHubHandler {
	return &GitHubHandler{
		engine: engine,
		secret: secret,
	}
}

// GitHubIssueEvent represents a GitHub issue webhook event
type GitHubIssueEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		State  string `json:"state"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	} `json:"issue"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// GitHubIssueCommentEvent represents a GitHub issue comment webhook event
type GitHubIssueCommentEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int `json:"number"`
	} `json:"issue"`
	Comment struct {
		Body string `json:"body"`
	} `json:"comment"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

// ServeHTTP handles incoming GitHub webhook requests
func (h *GitHubHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if h.secret != "" && !h.verifySignature(r, signature) {
		log.Println("Invalid GitHub webhook signature")
		http.Error(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	log.Printf("Received GitHub webhook: %s", eventType)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	switch eventType {
	case "issues":
		h.handleIssueEvent(body, w)
	case "issue_comment":
		h.handleIssueCommentEvent(body, w)
	case "ping":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	default:
		log.Printf("Unhandled event type: %s", eventType)
		w.WriteHeader(http.StatusOK)
	}
}

func (h *GitHubHandler) handleIssueEvent(body []byte, w http.ResponseWriter) {
	var event GitHubIssueEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Failed to parse issue event: %v", err)
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	// Check if this is a synced issue (to avoid loops)
	if h.isSyncedIssue(&event) {
		log.Printf("Skipping sync for already synced issue #%d", event.Issue.Number)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only sync on opened, edited, closed, or reopened actions
	if event.Action != "opened" && event.Action != "edited" && event.Action != "closed" && event.Action != "reopened" {
		log.Printf("Skipping action: %s", event.Action)
		w.WriteHeader(http.StatusOK)
		return
	}

	owner := event.Repository.Owner.Login
	repo := event.Repository.Name
	number := event.Issue.Number

	log.Printf("Processing GitHub issue %s/%s#%d (action: %s)", owner, repo, number, event.Action)

	// Sync to Jira
	if err := h.engine.SyncGitHubToJira(owner, repo, number); err != nil {
		log.Printf("Failed to sync GitHub issue to Jira: %v", err)
		http.Error(w, "Sync failed", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Synced successfully"))
}

func (h *GitHubHandler) handleIssueCommentEvent(body []byte, w http.ResponseWriter) {
	var event GitHubIssueCommentEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Failed to parse issue comment event: %v", err)
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	// For now, we'll just log comment events
	// In a full implementation, you could sync comments as well
	log.Printf("Received comment on issue #%d", event.Issue.Number)
	w.WriteHeader(http.StatusOK)
}

func (h *GitHubHandler) isSyncedIssue(event *GitHubIssueEvent) bool {
	for _, label := range event.Issue.Labels {
		if label.Name == "jira-sync" {
			return true
		}
	}
	return false
}

func (h *GitHubHandler) verifySignature(r *http.Request, signature string) bool {
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
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	expectedSignature := "sha256=" + expectedMAC

	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
