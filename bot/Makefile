SHELL := /bin/bash

GO              := go
GOOSE           := goose
SQLC            := sqlc
DATABASE_URL    ?= $(shell grep -E '^DATABASE_URL=' .env 2>/dev/null | cut -d= -f2-)
GOOSE_DIR       ?= db/migrations
HTTP_PORT       ?= 8080

GREEN := \033[0;32m
YELLOW:= \033[1;33m
RED   := \033[0;31m
NC    := \033[0m

.PHONY: help
help:
	@echo "$(GREEN)Rollton bot module$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS=":.*?## "}; {printf "  $(GREEN)%-22s$(NC) %s\n", $$1, $$2}'

## ---------- build / run ----------
.PHONY: build
build: ## Build all bot binaries into ./bin
	@mkdir -p bin
	@for d in cmd/*/; do \
		bot="$$(basename $$d)"; \
		echo "$(YELLOW)building $$bot$(NC)"; \
		$(GO) build -o bin/$$bot ./cmd/$$bot || exit 1; \
	done
	@echo "$(GREEN)✓ built$(NC)"

.PHONY: run
run: ## Run a bot locally (use: make run BOT=rolltonchatbot)
	@if [ -z "$(BOT)" ]; then echo "$(RED)BOT is required: make run BOT=rolltonchatbot$(NC)"; exit 1; fi
	@$(GO) run ./cmd/$(BOT)

.PHONY: dev
dev: ## Hot-reload run (use: make dev BOT=rolltonchatbot)
	@if [ -z "$(BOT)" ]; then echo "$(RED)BOT is required: make dev BOT=rolltonchatbot$(NC)"; exit 1; fi
	@BOT=$(BOT) air -c .air.toml

## ---------- test / lint ----------
.PHONY: test
test: ## Run unit tests (excludes integration)
	$(GO) test -race -short ./...

.PHONY: test-integration
test-integration: ## Run all tests including integration (testcontainers)
	$(GO) test -race ./...

.PHONY: coverage
coverage: ## Generate coverage report
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

.PHONY: lint
lint: ## go vet + gofmt check
	$(GO) vet ./...
	@diff -u <(echo -n) <(gofmt -l .)

.PHONY: tidy
tidy: ## go mod tidy
	$(GO) mod tidy

## ---------- migrations (goose) ----------
.PHONY: migrate-new
migrate-new: ## Create new migration (use: make migrate-new name=add_users)
	@if [ -z "$(name)" ]; then echo "$(RED)name is required: make migrate-new name=add_users$(NC)"; exit 1; fi
	$(GOOSE) -dir $(GOOSE_DIR) create $(name) sql

.PHONY: migrate-up
migrate-up: ## Apply pending migrations
	$(GOOSE) -dir $(GOOSE_DIR) postgres "$(DATABASE_URL)" up

.PHONY: migrate-down
migrate-down: ## Roll back one migration
	$(GOOSE) -dir $(GOOSE_DIR) postgres "$(DATABASE_URL)" down

.PHONY: migrate-status
migrate-status: ## Show migration status
	$(GOOSE) -dir $(GOOSE_DIR) postgres "$(DATABASE_URL)" status

.PHONY: migrate-reset
migrate-reset: ## Roll back everything
	$(GOOSE) -dir $(GOOSE_DIR) postgres "$(DATABASE_URL)" reset

## ---------- sqlc ----------
.PHONY: sqlc-gen
sqlc-gen: ## Regenerate sqlc code
	$(SQLC) generate

.PHONY: schema-dump
schema-dump: ## Regenerate db/schema/schema.sql from current migrations (uses throwaway pg container)
	@bash scripts/schema-dump.sh

## ---------- compose helpers ----------
.PHONY: up
up: ## Start postgres only
	docker compose up -d postgres

.PHONY: up-all
up-all: ## Start postgres + all bots
	docker compose --profile app up -d

.PHONY: down
down: ## Stop everything
	docker compose --profile app down

.PHONY: clean
clean: ## Remove containers + volumes
	docker compose --profile app down -v
	rm -rf bin coverage.out
