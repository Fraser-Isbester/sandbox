.PHONY: build run test clean deps

# Build the service
build:
	go build -o sync-service cmd/server/main.go

# Run the service
run:
	go run cmd/server/main.go

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -f sync-service
	rm -f sync_records.json

# Download dependencies
deps:
	go mod download
	go mod tidy

# Install dependencies
install: deps

# Run with example config
run-example:
	cp config.example.json config.json
	@echo "Please edit config.json with your credentials"
	@echo "Then run: make run"

# Format code
fmt:
	go fmt ./...

# Run linter (requires golangci-lint)
lint:
	golangci-lint run

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 go build -o sync-service-linux-amd64 cmd/server/main.go
	GOOS=darwin GOARCH=amd64 go build -o sync-service-darwin-amd64 cmd/server/main.go
	GOOS=windows GOARCH=amd64 go build -o sync-service-windows-amd64.exe cmd/server/main.go

# Docker build (future)
docker-build:
	docker build -t github-jira-sync:latest .

# Help
help:
	@echo "Available targets:"
	@echo "  build       - Build the service"
	@echo "  run         - Run the service"
	@echo "  test        - Run tests"
	@echo "  clean       - Clean build artifacts"
	@echo "  deps        - Download dependencies"
	@echo "  fmt         - Format code"
	@echo "  lint        - Run linter"
	@echo "  build-all   - Build for multiple platforms"
