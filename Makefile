# GoLogX — common dev tasks.
# Run `make help` for the full list.

.DEFAULT_GOAL := help
SHELL         := /bin/bash

VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
BIN_NAME := bin/logx

.PHONY: help
help: ## Show available targets.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: tidy
tidy: ## Sync go.mod / go.sum.
	go mod tidy

.PHONY: fmt
fmt: ## Format all Go source.
	gofmt -s -w .

.PHONY: vet
vet: ## Run go vet across all packages.
	go vet ./...

.PHONY: build
build: ## Build the cmd/logx CLI into ./$(BIN_NAME).
	@mkdir -p $(dir $(BIN_NAME))
	go build -ldflags='$(LDFLAGS)' -o $(BIN_NAME) ./cmd/logx

.PHONY: install
install: ## Install the CLI to $GOBIN.
	go install -ldflags='$(LDFLAGS)' ./cmd/logx

.PHONY: test
test: ## Run all tests.
	go test ./...

.PHONY: test-race
test-race: ## Run all tests with the race detector.
	go test -race -count=1 ./...

.PHONY: cover
cover: ## Coverage profile + HTML report.
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Open coverage.html in a browser."

.PHONY: bench
bench: ## Run benchmarks across all packages.
	go test -bench=. -benchmem -run=^$$ ./...

.PHONY: lint
lint: ## Run golangci-lint (install: https://golangci-lint.run/).
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed. Install: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	}
	golangci-lint run ./...

.PHONY: check
check: fmt vet test-race ## fmt + vet + race tests — the default sanity check.

.PHONY: snapshot
snapshot: ## Local snapshot release with goreleaser (no publish).
	@command -v goreleaser >/dev/null 2>&1 || { \
		echo "goreleaser not installed. Install: https://goreleaser.com/install/"; \
		exit 1; \
	}
	goreleaser release --snapshot --clean

.PHONY: clean
clean: ## Remove build & coverage artifacts.
	rm -f coverage.out coverage.html
	rm -rf bin/ dist/
	go clean ./...
