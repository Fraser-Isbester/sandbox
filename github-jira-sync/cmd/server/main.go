package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/config"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/github"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/jira"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/storage"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/sync"
	"github.com/fraser-isbester/sandbox/github-jira-sync/internal/webhook"
)

func main() {
	log.Println("Starting GitHub-Jira Sync Service")

	ctx := context.Background()

	// Load configuration
	cfg := loadConfig()

	// Initialize storage
	store, err := storage.NewFileStorage("sync_records.json")
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize GitHub client
	ghClient := github.NewClient(ctx, cfg.GitHub.Token)

	// Initialize Jira client
	jiraClient, err := jira.NewClient(ctx, cfg.Jira.URL, cfg.Jira.Username, cfg.Jira.APIToken)
	if err != nil {
		log.Fatalf("Failed to initialize Jira client: %v", err)
	}

	// Initialize sync engine
	engine := sync.NewEngine(ctx, cfg, ghClient, jiraClient, store)

	// Setup HTTP server with webhook handlers
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// GitHub webhook endpoint
	githubHandler := webhook.NewGitHubHandler(engine, cfg.GitHub.WebhookSecret)
	mux.Handle("/webhooks/github", githubHandler)

	// Jira webhook endpoint
	jiraHandler := webhook.NewJiraHandler(engine, cfg.Jira.WebhookSecret)
	mux.Handle("/webhooks/jira", jiraHandler)

	// Manual sync endpoints for testing
	mux.HandleFunc("/sync/github-to-jira", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		owner := r.URL.Query().Get("owner")
		repo := r.URL.Query().Get("repo")
		issueNumber := r.URL.Query().Get("issue")

		if owner == "" || repo == "" || issueNumber == "" {
			http.Error(w, "Missing parameters: owner, repo, issue", http.StatusBadRequest)
			return
		}

		var number int
		if _, err := fmt.Sscanf(issueNumber, "%d", &number); err != nil {
			http.Error(w, "Invalid issue number", http.StatusBadRequest)
			return
		}

		if err := engine.SyncGitHubToJira(owner, repo, number); err != nil {
			log.Printf("Sync failed: %v", err)
			http.Error(w, fmt.Sprintf("Sync failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Synced successfully"))
	})

	mux.HandleFunc("/sync/jira-to-github", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		jiraKey := r.URL.Query().Get("key")
		if jiraKey == "" {
			http.Error(w, "Missing parameter: key", http.StatusBadRequest)
			return
		}

		if err := engine.SyncJiraToGitHub(jiraKey); err != nil {
			log.Printf("Sync failed: %v", err)
			http.Error(w, fmt.Sprintf("Sync failed: %v", err), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Synced successfully"))
	})

	// List synced records endpoint
	mux.HandleFunc("/records", func(w http.ResponseWriter, r *http.Request) {
		records, err := store.ListRecords()
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to list records: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Simple JSON marshaling
		fmt.Fprintf(w, "[")
		for i, record := range records {
			if i > 0 {
				fmt.Fprintf(w, ",")
			}
			fmt.Fprintf(w, `{"github":"%s/%s#%d","jira":"%s","last_synced":%d}`,
				record.GitHubOwner, record.GitHubRepo, record.GitHubIssueNumber,
				record.JiraKey, record.LastSyncedAt)
		}
		fmt.Fprintf(w, "]")
	})

	// Create server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func loadConfig() *config.Config {
	// Try to load from file first
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			log.Printf("Warning: failed to load config from file: %v", err)
			log.Println("Falling back to environment variables")
		} else {
			log.Println("Configuration loaded from file")
			return cfg
		}
	}

	// Load from environment variables
	cfg := config.LoadConfigFromEnv()
	log.Println("Configuration loaded from environment variables")

	// Validate required fields
	if cfg.GitHub.Token == "" {
		log.Fatal("GITHUB_TOKEN is required")
	}
	if cfg.Jira.URL == "" {
		log.Fatal("JIRA_URL is required")
	}
	if cfg.Jira.Username == "" {
		log.Fatal("JIRA_USERNAME is required")
	}
	if cfg.Jira.APIToken == "" {
		log.Fatal("JIRA_API_TOKEN is required")
	}

	return cfg
}
