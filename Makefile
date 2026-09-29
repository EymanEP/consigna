# Consigna build tasks. Run `make help` for the list.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
BIN     := bin/consigna

.DEFAULT_GOAL := build
.PHONY: help build web web-deps run dev test test-go test-web lint lint-go lint-web fmt e2e check clean

help: ## Show this help
	@grep -E '^[a-z0-9-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "} {printf "  %-10s %s\n", $$1, $$2}'

web-deps: web/node_modules/.package-lock.json

web/node_modules/.package-lock.json: web/package.json web/package-lock.json
	npm --prefix web ci

web: web-deps ## Build the web UI into web/dist/app
	npm --prefix web run build

build: web ## Build the single binary with the UI embedded
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/consigna

run: build ## Build and run on port 7431
	./$(BIN)

dev: ## Run the Go server and the Vite dev server (UI on http://localhost:5173)
	@echo "API on :7431, UI with hot reload on http://localhost:5173"
	@trap 'kill 0' INT TERM; go run ./cmd/consigna --no-qr & npm --prefix web run dev; wait

test: test-go test-web ## Run all unit and integration tests

test-go:
	go test -race -count=1 ./...

test-web: web-deps
	npm --prefix web test

lint: lint-go lint-web ## Run all linters and type checks

lint-go:
	golangci-lint run ./...

lint-web: web-deps
	npm --prefix web run typecheck
	npm --prefix web run lint
	npm --prefix web run format:check

fmt: ## Format Go and web sources
	gofmt -w cmd internal web/embed.go
	npm --prefix web run format

e2e: build ## Run the browser end-to-end tests against the built binary
	npm --prefix web run e2e

check: lint test e2e ## Everything CI runs

clean: ## Remove build output
	rm -rf bin dist web/dist/app web/test-results web/playwright-report
