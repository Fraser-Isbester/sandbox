# Contributing to PubSub Scheduler

Thank you for your interest in contributing to PubSub Scheduler! This document provides guidelines and instructions for contributing.

## Development Setup

### Prerequisites

- **Go**: 1.21 or later
- **Python**: 3.8 or later
- **PostgreSQL**: 13 or later
- **Google Cloud SDK**: For Pub/Sub testing

### Local Development

1. Clone the repository:
```bash
git clone https://github.com/fraser-isbester/pubsubscheduler.git
cd pubsubscheduler
```

2. Set up database:
```bash
# Start PostgreSQL (via Docker)
docker run --name postgres-dev -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres:15

# Create database and run migrations
createdb scheduler
psql postgresql://postgres:postgres@localhost/scheduler < migrations/001_create_pubsub_scheduler.sql
```

3. Install dependencies:

**Go**:
```bash
cd go
go mod download
```

**Python**:
```bash
cd python
python -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
pip install -e .
pip install pytest pytest-cov
```

### Running Tests

**Go**:
```bash
cd go
go test ./pubsubscheduler/...
```

**Python**:
```bash
cd python
pytest
```

### Code Style

**Go**:
- Follow standard Go formatting (`gofmt`)
- Use `golangci-lint` for linting

**Python**:
- Follow PEP 8
- Use `black` for formatting
- Use `pylint` or `flake8` for linting

## Making Changes

### Branch Naming

- `feature/description` - New features
- `fix/description` - Bug fixes
- `docs/description` - Documentation changes
- `refactor/description` - Code refactoring

### Commit Messages

Follow conventional commits:
```
type(scope): description

[optional body]

[optional footer]
```

Examples:
```
feat(worker): add retry logic for failed messages
fix(scheduler): correct cron expression parsing
docs(readme): update installation instructions
```

### Pull Request Process

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Update documentation as needed
7. Submit a pull request

### Pull Request Template

```markdown
## Description
Brief description of changes

## Type of Change
- [ ] Bug fix
- [ ] New feature
- [ ] Breaking change
- [ ] Documentation update

## Testing
Describe testing performed

## Checklist
- [ ] Tests pass locally
- [ ] Code follows style guidelines
- [ ] Documentation updated
- [ ] Commits follow conventional commits
```

## Testing Guidelines

### Unit Tests

Write unit tests for:
- Core scheduling logic
- Message claiming (use test database)
- Cron expression parsing
- Worker behavior

Example (Python):
```python
def test_schedule_message():
    scheduler = Scheduler(db_url, mock_pubsub_client)
    message_id = scheduler.schedule_message(
        topic="test",
        message={"key": "value"},
        scheduled_at=datetime.now() + timedelta(hours=1)
    )
    assert message_id is not None
```

### Integration Tests

Test with real PostgreSQL and Pub/Sub emulator:

```bash
# Start Pub/Sub emulator
gcloud beta emulators pubsub start --project=test-project

# Run integration tests
export PUBSUB_EMULATOR_HOST=localhost:8085
pytest tests/integration/
```

## Documentation

### Code Documentation

- **Go**: Use godoc-style comments
- **Python**: Use docstrings (Google style)

Example (Python):
```python
def schedule_message(self, topic: str, message: Dict[str, Any], scheduled_at: datetime) -> str:
    """
    Schedule a single Pub/Sub message.

    Args:
        topic: Pub/Sub topic name
        message: Message payload as dictionary
        scheduled_at: When to publish the message

    Returns:
        Message ID (UUID)

    Raises:
        ValueError: If topic is invalid
        DatabaseError: If database operation fails
    """
```

### Documentation Files

Update relevant documentation:
- `README.md` - Main documentation
- `docs/comparison-to-cron.md` - Architecture comparisons
- `docs/10m-messages-example.md` - Usage examples

## Release Process

1. Update version in:
   - `go/go.mod`
   - `python/setup.py`
   - `python/pubsubscheduler/__init__.py`

2. Update CHANGELOG.md

3. Create a git tag:
```bash
git tag -a v0.2.0 -m "Release version 0.2.0"
git push origin v0.2.0
```

4. Create GitHub release with release notes

## Getting Help

- Open an issue for bugs or feature requests
- Join discussions in GitHub Discussions
- Reach out to maintainers for complex contributions

## Code of Conduct

### Our Pledge

We are committed to providing a welcoming and inclusive environment for all contributors.

### Expected Behavior

- Be respectful and inclusive
- Welcome newcomers
- Focus on constructive feedback
- Accept responsibility for mistakes

### Unacceptable Behavior

- Harassment or discrimination
- Trolling or insulting comments
- Personal or political attacks
- Publishing others' private information

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
