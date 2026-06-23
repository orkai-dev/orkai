# export PATH="$PATH:$(go env GOPATH)/bin"
# go install github.com/air-verse/air@latest
.PHONY: help dev dev-api dev-web dev-worker build build-api build-web migrate migrate-create fmt lint test test-api test-web test-cover test-integration test-integration-cover cover-gate clean docker-up docker-down swagger gen-api

# ============================================================================
# Variables
# ============================================================================

GO := go
GOFLAGS := -v
API_DIR := apps/api
WEB_DIR := apps/web
BIN_DIR := bin
COVERAGE_FILE := coverage.out

# ============================================================================
# Help
# ============================================================================

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ============================================================================
# Development
# ============================================================================

dev: docker-up ## Start full dev environment
	@$(MAKE) -j3 dev-api dev-web dev-worker

dev-api: ## Start Go API with hot reload
	cd $(API_DIR) && air

dev-web: ## Start Next.js dev server
	cd $(WEB_DIR) && bun dev

dev-worker: ## Start Go worker (PGMQ job consumer)
	cd $(API_DIR) && $(GO) run ./cmd/worker

# ============================================================================
# Build
# ============================================================================

build: build-api build-web ## Build all

build-api: ## Build Go API binary
	cd $(API_DIR) && $(GO) build $(GOFLAGS) -o ../../$(BIN_DIR)/orkai-api ./cmd/server
	cd $(API_DIR) && $(GO) build $(GOFLAGS) -o ../../$(BIN_DIR)/orkai-worker ./cmd/worker
	cd $(API_DIR) && $(GO) build $(GOFLAGS) -o ../../$(BIN_DIR)/orkai-migrate ./cmd/migrate

build-web: ## Build web production
	cd $(WEB_DIR) && bun run build

# ============================================================================
# Database
# ============================================================================

migrate: ## Run database migrations
	cd $(API_DIR) && $(GO) run ./cmd/migrate up

migrate-rollback: ## Rollback last migration
	cd $(API_DIR) && $(GO) run ./cmd/migrate rollback

migrate-create: ## Create new migration (usage: make migrate-create name=add_users)
	cd $(API_DIR) && $(GO) run ./cmd/migrate create $(name)

migrate-status: ## Show migration status
	cd $(API_DIR) && $(GO) run ./cmd/migrate status

# ============================================================================
# Format & Lint
# ============================================================================

fmt: ## Format & fix all code (Go + Biome)
	cd $(API_DIR) && gofmt -w -s . && goimports -w . 2>/dev/null || true
	bunx biome check --write $(WEB_DIR)

lint: ## Run all linters
	cd $(API_DIR) && golangci-lint run ./...
	bunx biome check $(WEB_DIR)

test: ## Run all tests
	cd $(API_DIR) && $(GO) test ./...
	cd $(WEB_DIR) && bun run test

test-api: ## Run Go tests only (with coverage profile)
	cd $(API_DIR) && $(GO) test -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	cd $(API_DIR) && $(GO) tool cover -func=$(COVERAGE_FILE) | tail -1

test-cover: test-api ## Run Go tests and open HTML coverage report
	cd $(API_DIR) && $(GO) tool cover -html=$(COVERAGE_FILE) -o coverage.html
	@echo "HTML coverage report: $(API_DIR)/coverage.html"

test-integration: ## Run integration tests (requires Docker; testcontainers + k8s fake)
	cd $(API_DIR) && $(GO) test -tags=integration ./...

test-integration-cover: ## Run all Go tests with coverage (requires Docker for integration)
	cd $(API_DIR) && $(GO) test -tags=integration -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic -timeout 15m ./...
	cd $(API_DIR) && $(GO) tool cover -func=$(COVERAGE_FILE) | tail -1

cover-gate: test-integration-cover ## Enforce per-package coverage floors on critical layers
	bash $(API_DIR)/scripts/coverage-gate.sh $(API_DIR)/$(COVERAGE_FILE)

test-web: ## Run web tests only
	cd $(WEB_DIR) && bun run test

# ============================================================================
# Docker (local dev infrastructure)
# ============================================================================

docker-up: ## Start dev infrastructure (PG, Valkey, K3s)
	docker compose -f deploy/docker-compose.yml up -d

docker-down: ## Stop dev infrastructure
	docker compose -f deploy/docker-compose.yml down

docker-destroy: ## Destroy dev infrastructure and volumes
	docker compose -f deploy/docker-compose.yml down -v

SWAG := $(shell go env GOPATH)/bin/swag

swagger: ## Generate OpenAPI spec from Go annotations (requires: go install github.com/swaggo/swag/cmd/swag@latest)
	cd $(API_DIR) && $(SWAG) init \
		-g internal/apidocs/doc.go \
		-o docs \
		--parseDependency --parseInternal \
		--outputTypes json,yaml

gen-api: swagger ## Copy OpenAPI spec into docs site
	cp $(API_DIR)/docs/swagger.json apps/docs/static/openapi.json
	bash $(API_DIR)/scripts/patch-openapi-servers.sh apps/docs/static/openapi.json
	cp apps/docs/static/openapi.json $(API_DIR)/docs/swagger.json

# ============================================================================
# Clean
# ============================================================================

clean: ## Clean build artifacts
	rm -rf $(BIN_DIR)
	cd $(API_DIR) && rm -f $(COVERAGE_FILE) coverage.html
	cd $(WEB_DIR) && rm -rf .next out node_modules/.cache
