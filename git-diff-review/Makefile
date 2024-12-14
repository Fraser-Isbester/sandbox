
build:
	mkdir -p ~/bin
	go build -o ~/bin/git-diff-review ./cmd/git-diff-review

test:
	@go test -v ./...

lint:
	@golangci-lint run
