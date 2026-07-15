# omniplug — developer Makefile
BINARY  := omniplug
PKG     := github.com/asingamaneni/omniplug
CMD     := ./cmd/omniplug
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X $(PKG)/internal/cli.version=$(VERSION)

# Dev tooling (override to pin versions, e.g. GORELEASER=...@v2.7.0)
GOLANGCI_LINT ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
GORELEASER    ?= github.com/goreleaser/goreleaser/v2@latest
HUGO_VERSION  ?= 0.163.3

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: setup
setup: ## One-time dev setup: deps, linter, and Hugo (extended)
	go mod download
	go install $(GOLANGCI_LINT)
	HUGO_VERSION=$(HUGO_VERSION) sh scripts/install-hugo.sh
	@echo "dev environment ready — ensure $$(go env GOPATH)/bin is on your PATH"
	@echo "to cut releases, also install GoReleaser: brew install goreleaser (or: go install $(GORELEASER))"

.PHONY: build
build: ## Build the binary into ./bin
	go build -ldflags '$(LDFLAGS)' -o bin/$(BINARY) $(CMD)

.PHONY: install
install: ## Install the binary into $(shell go env GOPATH)/bin
	go install -ldflags '$(LDFLAGS)' $(CMD)

.PHONY: run
run: ## Run against the example (ARGS="build -s examples/hello-plugin -o dist")
	go run $(CMD) $(ARGS)

.PHONY: demo
demo: build ## Inner loop: build, then validate + compile the example into ./dist
	./bin/$(BINARY) validate -s examples/hello-plugin
	./bin/$(BINARY) build -s examples/hello-plugin -o dist
	@echo "→ built dist/claude and dist/cursor from examples/hello-plugin"

.PHONY: test
test: ## Run tests
	go test ./...

.PHONY: test-race
test-race: ## Run tests with the race detector
	go test -race ./...

.PHONY: cover
cover: ## Run tests and print total coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: fmt
fmt: ## Format all Go code
	gofmt -w .

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: lint
lint: ## Run golangci-lint (https://golangci-lint.run/welcome/install)
	golangci-lint run

.PHONY: tidy
tidy: ## Tidy and verify go.mod
	go mod tidy

.PHONY: check
check: fmt vet test ## Format, vet, and test (pre-commit gate)

.PHONY: snapshot
snapshot: ## Build a local GoReleaser snapshot (no publish)
	goreleaser release --snapshot --clean

.PHONY: release-check
release-check: ## Validate the GoReleaser config
	goreleaser check

.PHONY: docs-gen
docs-gen: ## Regenerate the CLI command reference (Markdown) from the command tree
	go run $(CMD) gen-docs --dir site/content/docs/reference

.PHONY: docs
docs: docs-gen ## Build the static docs site into site/public (needs hugo)
	@command -v hugo >/dev/null 2>&1 || { echo "hugo not found — install with: brew install hugo"; exit 1; }
	cd site && hugo --gc --minify

.PHONY: docs-serve
docs-serve: docs-gen ## Serve the docs site locally at :1313 (needs hugo)
	@command -v hugo >/dev/null 2>&1 || { echo "hugo not found — install with: brew install hugo"; exit 1; }
	cd site && hugo serve

.PHONY: clean
clean: ## Remove build/test/docs artifacts
	rm -rf bin dist coverage.out site/public site/resources site/.hugo_build.lock
