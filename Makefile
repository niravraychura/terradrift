BINARY_NAME := terradrift
BIN_DIR := bin
GO_PACKAGES := ./...

.PHONY: build run test test-race fmt vet lint vuln ci clean docker-build help

build: ## Build the terradrift binary
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/terradrift

run: ## Run the CLI
	go run ./cmd/terradrift --help

test: ## Run unit tests
	go test $(GO_PACKAGES)

test-race: ## Run unit tests with the race detector
	go test -race $(GO_PACKAGES)

fmt: ## Format Go code
	gofmt -w $$(rg --files -g '*.go' -g '!vendor/**')

vet: ## Run go vet
	go vet $(GO_PACKAGES)

lint: ## Run golangci-lint
	golangci-lint run

vuln: ## Run govulncheck
	GOTOOLCHAIN=go1.26.5 go run golang.org/x/vuln/cmd/govulncheck@latest ./...

ci: ## Run the local CI quality gate without modifying files
	test -z "$$(gofmt -l $$(rg --files -g '*.go' -g '!vendor/**'))"
	$(MAKE) vet test test-race vuln lint

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR) dist coverage.out

docker-build: ## Build the Docker image
	docker build -t terradrift:local .

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "%-18s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
