.PHONY: build build-mcp build-all test vet lint check tools hooks install install-mcp clean

# Resolve golangci-lint whether or not $(go env GOPATH)/bin is on PATH.
GOLANGCI := $(shell command -v golangci-lint 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint)

build:
	go build -o bin/notion-pp-cli ./cmd/notion-pp-cli

build-mcp:
	go build -o bin/notion-pp-mcp ./cmd/notion-pp-mcp

build-all: build build-mcp

test:
	go test ./...

vet:
	go vet ./...

lint:
	@test -x "$(GOLANGCI)" || { echo "golangci-lint not found — run 'make tools' to install it"; exit 1; }
	$(GOLANGCI) run

# Run everything CI runs, in the same order. Use before every push.
check: vet test lint
	go build ./...
	@echo "✓ check passed — safe to push"

# Install dev tooling (golangci-lint v2, built with your local Go).
tools:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

# Enable the version-controlled git hooks (run once per clone).
hooks:
	git config core.hooksPath .githooks
	@echo "✓ git hooks enabled — pre-push will run 'make check'"

install:
	go install ./cmd/notion-pp-cli

install-mcp:
	go install ./cmd/notion-pp-mcp

clean:
	rm -rf bin/
