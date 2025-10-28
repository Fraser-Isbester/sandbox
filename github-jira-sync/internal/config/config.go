package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	GitHub GitHubConfig `json:"github"`
	Jira   JiraConfig   `json:"jira"`
	Sync   SyncConfig   `json:"sync"`
	Server ServerConfig `json:"server"`
}

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	Token        string `json:"token"`
	WebhookSecret string `json:"webhook_secret"`
}

// JiraConfig holds Jira-specific configuration
type JiraConfig struct {
	URL          string `json:"url"`
	Username     string `json:"username"`
	APIToken     string `json:"api_token"`
	WebhookSecret string `json:"webhook_secret"`
}

// SyncConfig holds sync mapping configuration
type SyncConfig struct {
	Mappings []ProjectMapping `json:"mappings"`
}

// ProjectMapping maps a Jira project to a GitHub repository
type ProjectMapping struct {
	JiraProject string   `json:"jira_project"`
	GitHubRepo  string   `json:"github_repo"`
	GitHubOwner string   `json:"github_owner"`
	SyncLabels  []string `json:"sync_labels"` // Optional labels to apply to synced issues
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

// LoadConfig loads configuration from a file
func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	return &config, nil
}

// LoadConfigFromEnv loads configuration from environment variables
func LoadConfigFromEnv() *Config {
	return &Config{
		GitHub: GitHubConfig{
			Token:        os.Getenv("GITHUB_TOKEN"),
			WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		},
		Jira: JiraConfig{
			URL:          os.Getenv("JIRA_URL"),
			Username:     os.Getenv("JIRA_USERNAME"),
			APIToken:     os.Getenv("JIRA_API_TOKEN"),
			WebhookSecret: os.Getenv("JIRA_WEBHOOK_SECRET"),
		},
		Server: ServerConfig{
			Port: 8080,
			Host: "0.0.0.0",
		},
	}
}

// GetMapping finds a mapping for a given Jira project
func (c *Config) GetMappingByJiraProject(project string) *ProjectMapping {
	for i := range c.Sync.Mappings {
		if c.Sync.Mappings[i].JiraProject == project {
			return &c.Sync.Mappings[i]
		}
	}
	return nil
}

// GetMappingByGitHubRepo finds a mapping for a given GitHub repository
func (c *Config) GetMappingByGitHubRepo(owner, repo string) *ProjectMapping {
	for i := range c.Sync.Mappings {
		if c.Sync.Mappings[i].GitHubOwner == owner && c.Sync.Mappings[i].GitHubRepo == repo {
			return &c.Sync.Mappings[i]
		}
	}
	return nil
}
