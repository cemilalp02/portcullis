# Requires GNU make. On Windows: `winget install ezwinports.make`, or run the
# commands below directly (they are plain `go` invocations).

GO            ?= go
GOLANGCI_LINT ?= golangci-lint
PKG           := github.com/cemilalp02/bekci
VERSION       ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT        ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
DATE          ?= $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)
LDFLAGS       := -s -w \
	-X $(PKG)/internal/version.version=$(VERSION) \
	-X $(PKG)/internal/version.commit=$(COMMIT) \
	-X $(PKG)/internal/version.date=$(DATE)
EXE           := $(if $(filter windows,$(shell $(GO) env GOOS)),.exe,)
BIN           := bin/bekci$(EXE)

.PHONY: all build test race cover lint fmt vuln tidy actionlint check clean

all: check build

build: ## Build a reproducible, static binary into ./bin
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/bekci

test: ## Run all tests
	$(GO) test ./...

race: ## Run all tests with the race detector (needs cgo)
	$(GO) test -race ./...

cover: ## Run tests with coverage and print a per-function summary
	$(GO) test -covermode=atomic -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

lint: ## Run golangci-lint (linters + formatting checks)
	$(GOLANGCI_LINT) run ./...

fmt: ## Format code
	$(GOLANGCI_LINT) fmt ./...

vuln: ## Check dependencies and stdlib for known vulnerabilities
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

tidy: ## Fail if go.mod / go.sum are not tidy
	$(GO) mod tidy -diff

actionlint: ## Lint GitHub workflow files
	$(GO) run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12

check: tidy lint test vuln actionlint ## Everything CI runs

clean:
	rm -rf bin coverage.out
