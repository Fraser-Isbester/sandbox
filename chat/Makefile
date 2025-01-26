.PHONY: generate
generate:
	@cd proto && buf generate

.PHONY: lint-proto
lint-proto:
	@cd proto && buf lint

.PHONY: lint-go
lint-go:
	@golangci-lint run

.PHONY: lint
lint: lint-go lint-proto

.PHONY: breaking
breaking:
	cd proto && buf breaking --against '.git#branch=main'

.PHONY: build
build:
	go build -o bin/server cmd/server/main.go

.PHONY: run
run: generate build
	./bin/server

.PHONY: test
test:
	go test ./...

.PHONY: test-curl
test-curl:
	buf curl \
	--http2-prior-knowledge \
	--data '{"message": "test"}' \
	--stream \
	http://localhost:50051/chat.v1.ChatService/Chat