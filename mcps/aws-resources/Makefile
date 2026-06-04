# AWS Resources — MCP server. Run these targets from this unit's directory.
.PHONY: help build test lint fmt vet run tidy

help: ## List targets
	@grep -hE '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*##"}{printf "  \033[36m%-8s\033[0m %s\n", $$1, $$2}'

build: ## Compile the server into ./bin
	go build -o bin/aws-resources ./cmd/aws-resources

test: ## Run tests with the race detector
	go test -race ./...

fmt: ## Format all Go files in place
	gofmt -w .

vet: ## Run go vet
	go vet ./...

lint: ## Check formatting, vet, and (if installed) staticcheck — non-mutating
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then echo "unformatted files:"; echo "$$unformatted"; exit 1; fi
	go vet ./...
	@command -v staticcheck >/dev/null 2>&1 \
		&& staticcheck ./... \
		|| echo "staticcheck not installed; skipping (go install honnef.co/go/tools/cmd/staticcheck@latest)"

run: ## Run the server over stdio
	go run ./cmd/aws-resources

tidy: ## Tidy go.mod / go.sum
	go mod tidy
