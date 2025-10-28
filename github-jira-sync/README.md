# GitHub-Jira Sync Service

A bidirectional synchronization service that maintains issue parity between GitHub and Jira. This POC allows you to manage issues in Jira (the management plane) while surfacing them in GitHub for developer visibility.

## Features

- **Bidirectional Sync**: Automatically syncs issues between GitHub and Jira
- **Webhook-Based**: Real-time updates using webhooks from both platforms
- **Flexible Mapping**: Configure multiple Jira projects to GitHub repositories
- **State Tracking**: Persistent storage tracks sync relationships
- **Manual Sync**: REST API endpoints for manual synchronization
- **Conflict Avoidance**: Detects synced issues to prevent sync loops

## Architecture

```
┌─────────┐                    ┌──────────────────┐                    ┌──────┐
│  Jira   │◄───────webhooks────┤  Sync Service    ├────webhooks───────►│GitHub│
└─────────┘                    │                  │                    └──────┘
                               │  ┌────────────┐  │
                               │  │Sync Engine │  │
                               │  └────────────┘  │
                               │  ┌────────────┐  │
                               │  │  Storage   │  │
                               │  └────────────┘  │
                               └──────────────────┘
```

## Components

### Core Packages

- **config**: Configuration management (file or environment-based)
- **github**: GitHub API client wrapper
- **jira**: Jira API client wrapper
- **sync**: Bidirectional sync engine
- **storage**: Issue relationship tracking
- **webhook**: Webhook handlers for both platforms

### Sync Engine

The sync engine maintains bidirectional synchronization:

1. **Jira → GitHub**: When a Jira issue is created/updated, it syncs to GitHub
2. **GitHub → Jira**: When a GitHub issue is created/updated, it syncs to Jira
3. **State Tracking**: Maintains mappings to prevent duplicate syncing
4. **Status Mapping**: Automatically maps issue states (open/closed)

## Configuration

### Environment Variables

```bash
# GitHub Configuration
export GITHUB_TOKEN="ghp_your_token_here"
export GITHUB_WEBHOOK_SECRET="your_webhook_secret"

# Jira Configuration
export JIRA_URL="https://your-domain.atlassian.net"
export JIRA_USERNAME="your-email@example.com"
export JIRA_API_TOKEN="your_jira_api_token"
export JIRA_WEBHOOK_SECRET="your_jira_webhook_secret"

# Optional: Load from config file
export CONFIG_PATH="/path/to/config.json"
```

### Configuration File (config.json)

```json
{
  "github": {
    "token": "ghp_your_token_here",
    "webhook_secret": "your_webhook_secret"
  },
  "jira": {
    "url": "https://your-domain.atlassian.net",
    "username": "your-email@example.com",
    "api_token": "your_jira_api_token",
    "webhook_secret": "your_jira_webhook_secret"
  },
  "sync": {
    "mappings": [
      {
        "jira_project": "PROJ",
        "github_repo": "my-repo",
        "github_owner": "my-org",
        "sync_labels": ["jira-synced"]
      }
    ]
  },
  "server": {
    "port": 8080,
    "host": "0.0.0.0"
  }
}
```

## Setup

### 1. Install Dependencies

```bash
cd github-jira-sync
go mod download
```

### 2. Configure Credentials

Create a GitHub personal access token with `repo` scope:
- Go to GitHub Settings → Developer settings → Personal access tokens
- Generate a new token with `repo` permissions

Create a Jira API token:
- Go to Atlassian Account Settings → Security → API tokens
- Create a new API token

### 3. Configure Webhooks

**GitHub Webhook:**
- Go to your repository → Settings → Webhooks
- Add webhook URL: `http://your-server:8080/webhooks/github`
- Content type: `application/json`
- Secret: Your webhook secret
- Events: Issues, Issue comments

**Jira Webhook:**
- Go to Jira → System → Webhooks
- Create webhook URL: `http://your-server:8080/webhooks/jira`
- Events: Issue Created, Issue Updated, Issue Deleted

### 4. Run the Service

```bash
# Using environment variables
go run cmd/server/main.go

# Using config file
CONFIG_PATH=config.json go run cmd/server/main.go
```

## API Endpoints

### Health Check
```bash
GET /health
```

### Manual Sync: GitHub → Jira
```bash
POST /sync/github-to-jira?owner=my-org&repo=my-repo&issue=123
```

### Manual Sync: Jira → GitHub
```bash
POST /sync/jira-to-github?key=PROJ-123
```

### List Sync Records
```bash
GET /records
```

Returns all synced issue pairs:
```json
[
  {
    "github": "my-org/my-repo#123",
    "jira": "PROJ-456",
    "last_synced": 1234567890
  }
]
```

## Usage Examples

### Example 1: Create Jira issue, sync to GitHub

1. Create a Jira issue in project `PROJ`
2. Service receives webhook from Jira
3. Service creates corresponding GitHub issue
4. Service stores sync record
5. GitHub issue includes link to Jira ticket

### Example 2: Update GitHub issue, sync to Jira

1. Developer updates GitHub issue #123
2. Service receives webhook from GitHub
3. Service updates corresponding Jira issue
4. Both issues stay in sync

### Example 3: Close issue from either side

1. Issue is closed in Jira
2. Service receives webhook
3. Service closes corresponding GitHub issue
4. Vice versa works as well

## Sync Behavior

### Issue Creation
- **Jira → GitHub**: Creates new GitHub issue with label "jira-sync"
- **GitHub → Jira**: Creates new Jira issue (type: Task)
- Both directions include backlinks in the description

### Issue Updates
- Title/summary changes sync bidirectionally
- Description/body changes sync bidirectionally
- Status changes (open/closed) sync bidirectionally

### Sync Prevention
- Issues with "jira-sync" label are detected to prevent loops
- Descriptions containing "Synced from" are recognized as synced
- Only mapped projects/repos are synced

## Development

### Build
```bash
go build -o sync-service cmd/server/main.go
```

### Run Tests
```bash
go test ./...
```

### Project Structure
```
github-jira-sync/
├── cmd/
│   └── server/
│       └── main.go          # Main entry point
├── internal/
│   ├── config/
│   │   └── config.go        # Configuration management
│   ├── github/
│   │   └── client.go        # GitHub API client
│   ├── jira/
│   │   └── client.go        # Jira API client
│   ├── storage/
│   │   └── storage.go       # Sync record storage
│   ├── sync/
│   │   └── engine.go        # Bidirectional sync engine
│   └── webhook/
│       ├── github.go        # GitHub webhook handler
│       └── jira.go          # Jira webhook handler
├── go.mod
└── README.md
```

## Limitations & Future Enhancements

### Current Limitations
- Comment synchronization not implemented
- Attachments not synced
- Custom fields not mapped
- No conflict resolution for simultaneous updates
- File-based storage (not production-ready)

### Future Enhancements
- [ ] Sync comments between platforms
- [ ] Support for attachments
- [ ] Custom field mapping
- [ ] Database storage (PostgreSQL, MySQL)
- [ ] Conflict resolution strategies
- [ ] Label synchronization
- [ ] Assignee mapping
- [ ] Priority/severity mapping
- [ ] Metrics and monitoring
- [ ] Rate limiting and retry logic

## Troubleshooting

### Issue: Webhooks not received
- Check that your server is publicly accessible
- Verify webhook URLs in GitHub/Jira settings
- Check webhook secrets match configuration
- Review server logs for errors

### Issue: Sync loops (duplicate syncing)
- Ensure "jira-sync" label is applied to synced issues
- Check that descriptions contain sync markers
- Review sync records in `sync_records.json`

### Issue: Authentication failures
- Verify GitHub token has correct permissions
- Ensure Jira API token is valid
- Check Jira URL format (include https://)

## License

This is a proof-of-concept implementation. Adjust according to your needs.

## Contributing

This is a POC. For production use, consider:
- Adding comprehensive error handling
- Implementing database storage
- Adding metrics and monitoring
- Improving conflict resolution
- Adding comprehensive tests
