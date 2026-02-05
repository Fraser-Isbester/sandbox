.PHONY: help setup-db test-go test-python test docker-build-go docker-build-python clean

help:
	@echo "PubSub Scheduler - Development Commands"
	@echo ""
	@echo "Database:"
	@echo "  make setup-db          - Create database and run migrations"
	@echo "  make reset-db          - Drop and recreate database"
	@echo ""
	@echo "Testing:"
	@echo "  make test              - Run all tests (Go + Python)"
	@echo "  make test-go           - Run Go tests"
	@echo "  make test-python       - Run Python tests"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-build-go   - Build Go worker Docker image"
	@echo "  make docker-build-python - Build Python worker Docker image"
	@echo ""
	@echo "Development:"
	@echo "  make run-worker-go     - Run Go worker locally"
	@echo "  make run-worker-python - Run Python worker locally"
	@echo ""
	@echo "Cleanup:"
	@echo "  make clean             - Remove build artifacts"

# Database setup
setup-db:
	@echo "Creating database and running migrations..."
	createdb scheduler || true
	psql postgresql://localhost/scheduler < migrations/001_create_pubsub_scheduler.sql
	@echo "✓ Database ready"

reset-db:
	@echo "Resetting database..."
	dropdb scheduler --if-exists
	createdb scheduler
	psql postgresql://localhost/scheduler < migrations/001_create_pubsub_scheduler.sql
	@echo "✓ Database reset complete"

# Testing
test: test-go test-python

test-go:
	@echo "Running Go tests..."
	cd go && go test ./pubsubscheduler/... -v

test-python:
	@echo "Running Python tests..."
	cd python && python -m pytest -v

# Docker builds
docker-build-go:
	@echo "Building Go worker Docker image..."
	docker build -t pubsub-scheduler-worker:latest -f go/examples/Dockerfile .
	@echo "✓ Go worker image built"

docker-build-python:
	@echo "Building Python worker Docker image..."
	docker build -t pubsub-scheduler-worker-python:latest -f python/examples/Dockerfile .
	@echo "✓ Python worker image built"

# Local development
run-worker-go:
	@echo "Running Go worker locally..."
	cd go/examples && go run main.go

run-worker-python:
	@echo "Running Python worker locally..."
	cd python/examples && python main.py

# Cleanup
clean:
	@echo "Cleaning up..."
	rm -rf go/pubsubscheduler/*.test
	rm -rf python/build python/dist python/*.egg-info
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
	find . -type f -name "*.pyc" -delete
	@echo "✓ Cleanup complete"
