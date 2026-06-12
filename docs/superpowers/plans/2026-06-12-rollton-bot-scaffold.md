# Rollton `bot/` Scaffold — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold the `bot/` directory of the `rollton` monorepo into a buildable, runnable shape — multiple Telegram bot binaries sharing a hex-arch Go codebase with goose migrations, sqlc, and a Postgres-backed dev compose stack. No real domain code yet.

**Architecture:** Per-bot vertical slice under `internal/bots/<name>/` (handlers + bot-specific services) over a shared `internal/core/{domain,ports,services}` and shared `pkg/{http,tgbot,sqlc/postgres}`. Each `cmd/bot-X/main.go` is a thin composition root that builds its bot via `internal/bots/<name>.NewApp(...)` and runs Telegram long-poll + an HTTP server (healthz now, REST later) under one `errgroup`.

**Tech Stack:** Go 1.22+ (stdlib `slog`), `github.com/go-telegram-bot-api/telegram-bot-api/v5`, `github.com/pressly/goose/v3`, `sqlc`, `pgx/v5/stdlib`, `golang.org/x/sync/errgroup`, `github.com/joho/godotenv` (dev), `github.com/stretchr/testify/require`. PostgreSQL via Docker Compose. `air` for dev hot-reload.

**Reference spec:** `docs/superpowers/specs/2026-06-12-rollton-bot-structure-design.md`.

---

## Pre-flight (do once, before Task 1)

These values come from the executor; the plan uses placeholders that must be replaced consistently across all tasks.

| Placeholder | Default | Replace with |
|---|---|---|
| `<OWNER>` | `kotov-const` | The GitHub org/user that will host this repo. Used in `module github.com/<OWNER>/rollton/bot`. |
| `<BOT_A>` | `bot-a` | Real name of the first bot (kebab-case for binary, used as `<BOT_A_PKG>` for the Go package). |
| `<BOT_A_PKG>` | `bot_a` | Same name in snake_case (Go package — no hyphens allowed). |
| `<BOT_B>` / `<BOT_B_PKG>` | `bot-b` / `bot_b` | Second bot. |

If the executor does not have real names yet, leave the defaults — renaming is a single `sed` later because every reference is namespaced.

---

## File Structure (what each file owns)

```
rollton/                                               # monorepo root
├── README.md                                          # monorepo overview
├── Makefile                                           # delegates to bot/ and infra/
├── .gitignore                                         # repo-wide ignores
├── docs/superpowers/                                  # already exists (spec + plan)
├── bot/
│   ├── go.mod  go.sum                                 # module: github.com/<OWNER>/rollton/bot
│   ├── Dockerfile                                     # multi-stage; ARG BOT picks cmd/<BOT>
│   ├── docker-compose.yml                             # postgres + each bot service
│   ├── Makefile                                       # build/run/test/lint/migrate/sqlc
│   ├── sqlc.yaml                                      # sqlc config
│   ├── .air.toml                                      # hot-reload (BOT env picks binary)
│   ├── .env.example                                   # documents required env vars
│   ├── README.md                                      # how to run bots locally
│   ├── cmd/
│   │   ├── <BOT_A>/main.go                            # composition root for <BOT_A>
│   │   └── <BOT_B>/main.go                            # composition root for <BOT_B>
│   ├── internal/
│   │   ├── config/config.go                           # env loader → Config struct
│   │   ├── config/config_test.go                      # tests for Load()
│   │   ├── middleware/logging.go                      # slog logger + tgbot/http middleware
│   │   ├── testutil/db.go                             # placeholder; will hold NewTestDB later
│   │   ├── bots/<BOT_A_PKG>/
│   │   │   ├── wire.go                                # Deps, App, NewApp, (*App).Run(ctx)
│   │   │   ├── handlers/telegram/start_handler.go     # /start → static reply
│   │   │   ├── handlers/telegram/test/start_test.go   # tests start handler
│   │   │   └── handlers/http/healthz_handler.go       # GET /healthz → 200
│   │   ├── bots/<BOT_B_PKG>/                          # mirror of <BOT_A_PKG>
│   │   └── core/
│   │       ├── domain/.gitkeep                        # empty for now
│   │       ├── domain/test/.gitkeep
│   │       ├── ports/.gitkeep
│   │       ├── services/.gitkeep
│   │       └── services/test/.gitkeep
│   ├── pkg/
│   │   ├── http/response.go                           # net/http JSON response helpers
│   │   ├── http/errors.go                             # error code constants
│   │   ├── http/response_test.go
│   │   ├── tgbot/bot.go                               # New, Run (long-poll)
│   │   ├── tgbot/context.go                           # Context (UserID, ChatID, Reply, …)
│   │   ├── tgbot/router.go                            # Handle/HandleCallback/Dispatch
│   │   ├── tgbot/middleware.go                        # Middleware, Chain
│   │   ├── tgbot/router_test.go
│   │   └── sqlc/postgres/.gitkeep                     # empty until first sqlc gen
│   └── db/
│       ├── migrations/.gitkeep                        # goose .sql files (none yet)
│       ├── queries/.gitkeep                           # sqlc input (none yet)
│       └── schema/schema.sql                          # empty file; sqlc reads this
└── infra/
    └── README.md                                      # placeholder
```

---

## Task 1: Bootstrap the monorepo

**Files:**
- Create: `rollton/.gitignore`, `rollton/README.md`, `rollton/Makefile`, `rollton/infra/README.md`

- [ ] **Step 1: `git init`**

Run:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git init -b main
```
Expected: `Initialized empty Git repository in .../rollton/.git/`.

- [ ] **Step 2: Write `.gitignore`**

Create `rollton/.gitignore`:
```gitignore
# Editors
.idea/
.DS_Store
*.swp

# Go
*.test
*.out
coverage.out
bot/bin/

# Env / secrets
.env
.env.*.local

# Build artifacts
dist/
```

- [ ] **Step 3: Write `README.md`**

Create `rollton/README.md`:
```markdown
# Rollton

Monorepo for the Rollton Telegram bot platform.

| Dir | Purpose |
|---|---|
| `bot/` | Go module with all Telegram bot binaries (`cmd/<bot>/`) and shared internals. |
| `infra/` | Terraform for AWS resources. Placeholder until infra is needed. |
| `docs/superpowers/` | Specs and implementation plans (managed by superpowers workflow). |

See `bot/README.md` for local development instructions.
```

- [ ] **Step 4: Write root `Makefile`**

Create `rollton/Makefile`:
```make
.PHONY: help bot infra

help:
	@echo "Rollton monorepo — delegate to subprojects:"
	@echo "  make -C bot <target>      # see bot/Makefile"
	@echo "  make -C infra <target>    # see infra/ (placeholder)"

bot:
	@$(MAKE) -C bot $(filter-out $@,$(MAKECMDGOALS))

infra:
	@$(MAKE) -C infra $(filter-out $@,$(MAKECMDGOALS))

%:
	@:
```

- [ ] **Step 5: Write `infra/README.md`**

Create `rollton/infra/README.md`:
```markdown
# infra/

Terraform configuration for Rollton infrastructure. Empty placeholder — populated when first cloud resources are needed.
```

- [ ] **Step 6: First commit (includes the already-written spec + plan)**

Run:
```bash
git add .gitignore README.md Makefile infra/README.md docs/
git status
git commit -m "chore: bootstrap monorepo with spec and plan"
```
Expected: commit succeeds; `git status` shows clean tree.

---

## Task 2: Initialize `bot/` Go module

**Files:**
- Create: `bot/go.mod`, `bot/cmd/<BOT_A>/main.go` (placeholder)

- [ ] **Step 1: `go mod init`**

Run:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go mod init github.com/<OWNER>/rollton/bot
```
Expected: `bot/go.mod` is created with `module github.com/<OWNER>/rollton/bot` and a `go 1.22` (or higher) directive.

- [ ] **Step 2: Add a placeholder `main.go` so `go build ./...` works**

Create `bot/cmd/<BOT_A>/main.go`:
```go
package main

func main() {}
```

- [ ] **Step 3: Verify the module builds**

Run:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go build ./...
```
Expected: exits 0, no output.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/
git commit -m "feat(bot): initialize go module"
```

---

## Task 3: Create skeleton directory tree

**Files:**
- Create directories and `.gitkeep` files for every leaf dir in the spec layout.

- [ ] **Step 1: Create all directories at once**

Run from `rollton/`:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton

mkdir -p \
  bot/cmd/<BOT_A> \
  bot/cmd/<BOT_B> \
  bot/internal/bots/<BOT_A_PKG>/handlers/telegram/test \
  bot/internal/bots/<BOT_A_PKG>/handlers/http/test \
  bot/internal/bots/<BOT_A_PKG>/services/test \
  bot/internal/bots/<BOT_B_PKG>/handlers/telegram/test \
  bot/internal/bots/<BOT_B_PKG>/handlers/http/test \
  bot/internal/bots/<BOT_B_PKG>/services/test \
  bot/internal/core/domain/test \
  bot/internal/core/ports \
  bot/internal/core/services/test \
  bot/internal/config \
  bot/internal/middleware \
  bot/internal/testutil \
  bot/pkg/http \
  bot/pkg/tgbot \
  bot/pkg/sqlc/postgres/test \
  bot/db/migrations \
  bot/db/queries \
  bot/db/schema \
  bot/docs
```

- [ ] **Step 2: Drop `.gitkeep` into every directory that does not yet contain a file**

Run from `rollton/`:
```bash
find bot -type d -empty -exec touch {}/.gitkeep \;
```
Expected: `.gitkeep` placed in every still-empty dir (Go won't keep empty dirs in git).

- [ ] **Step 3: Verify**

```bash
find bot -type d | sort
```
Expected: every directory in the file-structure section above is present.

- [ ] **Step 4: Commit**

```bash
git add bot/
git commit -m "chore(bot): scaffold directory layout"
```

---

## Task 4: Docker assets

**Files:**
- Create: `bot/Dockerfile`, `bot/docker-compose.yml`, `bot/.dockerignore`, `bot/.env.example`

- [ ] **Step 1: Write `bot/Dockerfile`**

Create `bot/Dockerfile`:
```dockerfile
# syntax=docker/dockerfile:1.6
ARG GO_VERSION=1.22

FROM golang:${GO_VERSION}-alpine AS builder
ARG BOT
RUN apk add --no-cache git
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN test -n "$BOT" || (echo "ERROR: --build-arg BOT=<bot-name> is required" && exit 1)
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/bot ./cmd/${BOT}

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/bot /usr/local/bin/bot
USER 65532:65532
ENTRYPOINT ["/usr/local/bin/bot"]
```

- [ ] **Step 2: Write `bot/.dockerignore`**

Create `bot/.dockerignore`:
```
.git
bin/
*.test
coverage.out
.env
.env.*
.idea/
.vscode/
```

- [ ] **Step 3: Write `bot/.env.example`**

Create `bot/.env.example`:
```dotenv
# Postgres
POSTGRES_USER=rollton
POSTGRES_PASSWORD=rollton_pass
POSTGRES_DB=rollton
POSTGRES_PORT=5432
DATABASE_URL=postgres://rollton:rollton_pass@localhost:5432/rollton?sslmode=disable

# HTTP
HTTP_PORT=8080

# Logging
LOG_LEVEL=info
LOG_FORMAT=json

# Telegram tokens (one per bot)
TOKEN_<BOT_A_UPPER>=replace_with_real_token
TOKEN_<BOT_B_UPPER>=replace_with_real_token
```
(Replace `<BOT_A_UPPER>` with the uppercase, underscored bot name, e.g. `TOKEN_BOT_A`.)

- [ ] **Step 4: Write `bot/docker-compose.yml`**

Create `bot/docker-compose.yml`:
```yaml
services:
  postgres:
    image: postgres:16-alpine
    container_name: rollton-postgres
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-rollton}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD:-rollton_pass}
      POSTGRES_DB: ${POSTGRES_DB:-rollton}
    ports:
      - "${POSTGRES_PORT:-5432}:5432"
    volumes:
      - rollton_pg:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${POSTGRES_USER:-rollton}"]
      interval: 5s
      timeout: 5s
      retries: 10

  <BOT_A>:
    build:
      context: .
      args:
        BOT: <BOT_A>
    container_name: rollton-<BOT_A>
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://${POSTGRES_USER:-rollton}:${POSTGRES_PASSWORD:-rollton_pass}@postgres:5432/${POSTGRES_DB:-rollton}?sslmode=disable
      HTTP_PORT: 8080
      LOG_LEVEL: ${LOG_LEVEL:-info}
      LOG_FORMAT: ${LOG_FORMAT:-json}
      TELEGRAM_TOKEN: ${TOKEN_<BOT_A_UPPER>}
    ports:
      - "8080:8080"
    profiles: ["app"]

  <BOT_B>:
    build:
      context: .
      args:
        BOT: <BOT_B>
    container_name: rollton-<BOT_B>
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      DATABASE_URL: postgres://${POSTGRES_USER:-rollton}:${POSTGRES_PASSWORD:-rollton_pass}@postgres:5432/${POSTGRES_DB:-rollton}?sslmode=disable
      HTTP_PORT: 8081
      LOG_LEVEL: ${LOG_LEVEL:-info}
      LOG_FORMAT: ${LOG_FORMAT:-json}
      TELEGRAM_TOKEN: ${TOKEN_<BOT_B_UPPER>}
    ports:
      - "8081:8081"
    profiles: ["app"]

volumes:
  rollton_pg:
```
(Replace `<BOT_A_UPPER>`, `<BOT_B_UPPER>` as in the `.env.example` step.)

- [ ] **Step 5: Verify postgres comes up**

Run:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
cp .env.example .env
docker compose up -d postgres
docker compose ps
```
Expected: `postgres` is `running (healthy)` within ~10s.

- [ ] **Step 6: Tear down and commit**

```bash
docker compose down
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/Dockerfile bot/.dockerignore bot/docker-compose.yml bot/.env.example
git commit -m "feat(bot): add docker compose and Dockerfile"
```

---

## Task 5: `bot/Makefile` and `bot/.air.toml`

**Files:**
- Create: `bot/Makefile`, `bot/.air.toml`

- [ ] **Step 1: Write `bot/Makefile`**

Create `bot/Makefile`:
```make
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
run: ## Run a bot locally (use: make run BOT=bot-a)
	@if [ -z "$(BOT)" ]; then echo "$(RED)BOT is required: make run BOT=bot-a$(NC)"; exit 1; fi
	@$(GO) run ./cmd/$(BOT)

.PHONY: dev
dev: ## Hot-reload run (use: make dev BOT=bot-a)
	@if [ -z "$(BOT)" ]; then echo "$(RED)BOT is required: make dev BOT=bot-a$(NC)"; exit 1; fi
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
```

- [ ] **Step 2: Write `bot/.air.toml`**

Create `bot/.air.toml`:
```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/bot ./cmd/${BOT}"
  bin = "./tmp/bot"
  include_ext = ["go"]
  exclude_dir = ["tmp", "bin", "db"]
  delay = 500

[log]
  time = true

[misc]
  clean_on_exit = true
```

- [ ] **Step 3: Write `bot/scripts/schema-dump.sh`**

Create directory + script:
```bash
mkdir -p /Users/konstantinkotau/Desktop/projects.com/rollton/bot/scripts
```
Create `bot/scripts/schema-dump.sh`:
```bash
#!/usr/bin/env bash
# Regenerate db/schema/schema.sql by running every goose migration against a
# throwaway Postgres container and dumping the resulting schema.
set -euo pipefail

CONTAINER=rollton-schema-dump-$$
PORT=55432
PASSWORD=schema_dump_pass

cleanup() { docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; }
trap cleanup EXIT

docker run --rm -d \
  --name "$CONTAINER" \
  -e POSTGRES_PASSWORD="$PASSWORD" \
  -e POSTGRES_DB=schema_dump \
  -p "$PORT":5432 \
  postgres:16-alpine >/dev/null

# wait for ready
for _ in {1..30}; do
  if docker exec "$CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done

URL="postgres://postgres:${PASSWORD}@localhost:${PORT}/schema_dump?sslmode=disable"

if compgen -G "db/migrations/*.sql" >/dev/null; then
  goose -dir db/migrations postgres "$URL" up
fi

docker exec "$CONTAINER" pg_dump -U postgres --schema-only --no-owner --no-privileges schema_dump \
  > db/schema/schema.sql

echo "✓ db/schema/schema.sql regenerated"
```
Make it executable:
```bash
chmod +x /Users/konstantinkotau/Desktop/projects.com/rollton/bot/scripts/schema-dump.sh
```

- [ ] **Step 4: Verify `make help` works**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
make help
```
Expected: prints the colored list of targets including `schema-dump`.

- [ ] **Step 5: Verify `make build` works (still builds the empty placeholder)**

```bash
make build
ls bin/
```
Expected: `bin/<BOT_A>` exists. Plain stdout.

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/Makefile bot/.air.toml bot/scripts/schema-dump.sh
git commit -m "feat(bot): add Makefile, air config, schema-dump script"
```

---

## Task 6: Add goose dependency and document the CLI

**Files:**
- Modify: `bot/go.mod`, `bot/go.sum`
- Create: `bot/README.md`

- [ ] **Step 1: Add the goose library**

Run:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go get github.com/pressly/goose/v3@latest
go get github.com/jackc/pgx/v5/stdlib@latest
```
Expected: dependencies added to `go.mod`, downloaded to module cache.

- [ ] **Step 2: Install the goose CLI (host-side, documented; not required for build)**

Document in `bot/README.md` (Step 3). To install now for verification:
```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
goose -version
```
Expected: prints a version string.

- [ ] **Step 3: Write `bot/README.md`**

Create `bot/README.md`:
```markdown
# bot/

Go module for all Rollton Telegram bots. One module, multiple binaries (`cmd/<bot>/`), shared internals under `internal/`.

## Prerequisites

- Go 1.22+
- Docker & Docker Compose
- `goose` CLI: `go install github.com/pressly/goose/v3/cmd/goose@latest`
- `sqlc`:     `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`
- `air`:      `go install github.com/air-verse/air@latest` (hot reload, dev only)

## First-time setup

```bash
cp .env.example .env                   # fill in TOKEN_BOT_A etc.
make up                                # start postgres
make migrate-status                    # should report "no migrations" cleanly
make build                             # build all binaries into ./bin/
```

## Running a bot

```bash
make run BOT=bot-a                     # vanilla
make dev BOT=bot-a                     # with hot reload via air
```

## Migrations (goose)

```bash
make migrate-new name=add_users        # creates db/migrations/<timestamp>_add_users.sql
make migrate-up                        # apply
make migrate-status
make migrate-down                      # roll back one step
```

After modifying migrations, regenerate the snapshot schema sqlc reads:

```bash
make schema-dump                       # uses scripts/schema-dump.sh
```

## sqlc

```bash
make sqlc-gen                          # writes to pkg/sqlc/postgres/
```

## Tests

```bash
make test                              # unit only (-short)
make test-integration                  # includes testcontainers
make coverage
```
```

- [ ] **Step 4: Verify `make migrate-status` runs cleanly against the dev DB**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
docker compose up -d postgres
sleep 3
set -a; source .env; set +a
make migrate-status
```
Expected: output like `Applied At                  Migration` with no rows (no migrations yet). Exits 0.

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/go.mod bot/go.sum bot/README.md
git commit -m "feat(bot): add goose and pgx dependencies, write README"
```

---

## Task 7: sqlc configuration

**Files:**
- Create: `bot/sqlc.yaml`, `bot/db/schema/schema.sql` (empty)

- [ ] **Step 1: Write `bot/sqlc.yaml`**

Create `bot/sqlc.yaml`:
```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "db/schema/schema.sql"
    queries: "db/queries"
    gen:
      go:
        package: "postgres"
        out: "pkg/sqlc/postgres"
        sql_package: "pgx/v5"
        emit_interface: true
        emit_json_tags: true
        emit_prepared_queries: false
        emit_exact_table_names: false
```

- [ ] **Step 2: Create an empty `bot/db/schema/schema.sql`**

Create `bot/db/schema/schema.sql`:
```sql
-- Canonical schema snapshot. Regenerated from db/migrations by `make schema-dump`.
-- Intentionally empty until the first migration is added.
```

- [ ] **Step 3: Verify `sqlc generate` runs without error (no-op since no queries exist)**

Install sqlc if not present:
```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```
Then:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
make sqlc-gen
```
Expected: exits 0 with no generated files (nothing in `pkg/sqlc/postgres/` apart from `.gitkeep`).

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/sqlc.yaml bot/db/schema/schema.sql
git commit -m "feat(bot): add sqlc config and empty schema snapshot"
```

---

## Task 8: `pkg/http` (net/http response helpers)

**Files:**
- Create: `bot/pkg/http/response.go`, `bot/pkg/http/errors.go`, `bot/pkg/http/response_test.go`
- Delete: `bot/pkg/http/.gitkeep` (replaced by real files)

- [ ] **Step 1: Write the failing test**

Create `bot/pkg/http/response_test.go`:
```go
package http_test

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	httpx "github.com/<OWNER>/rollton/bot/pkg/http"
	"github.com/stretchr/testify/require"
)

func TestWriteSuccess(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteSuccess(rec, map[string]string{"hello": "world"})

	require.Equal(t, 200, rec.Code)
	require.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var got httpx.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.True(t, got.Success)
	require.Nil(t, got.Error)
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	httpx.WriteError(rec, 400, httpx.ErrCodeInvalidRequest, "bad body")

	require.Equal(t, 400, rec.Code)
	var got httpx.Response
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &got))
	require.False(t, got.Success)
	require.Equal(t, httpx.ErrCodeInvalidRequest, got.Error.Code)
	require.Equal(t, "bad body", got.Error.Message)
}
```

- [ ] **Step 2: Add testify**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go get github.com/stretchr/testify@latest
```

- [ ] **Step 3: Run test, watch it fail**

```bash
go test ./pkg/http/... -run TestWriteSuccess -v
```
Expected: compile error — `WriteSuccess`, `Response`, `ErrCodeInvalidRequest` undefined.

- [ ] **Step 4: Write `bot/pkg/http/response.go`**

Replace `bot/pkg/http/.gitkeep` with `bot/pkg/http/response.go`:
```go
// Package http provides JSON response helpers built on net/http.
package http

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func WriteJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func WriteSuccess(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusOK, Response{Success: true, Data: data})
}

func WriteCreated(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, Response{Success: true, Data: data})
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, Response{
		Success: false,
		Error:   &ErrorInfo{Code: code, Message: message},
	})
}
```

- [ ] **Step 5: Write `bot/pkg/http/errors.go`**

```go
package http

const (
	ErrCodeInvalidRequest = "INVALID_REQUEST"
	ErrCodeInternalError  = "INTERNAL_ERROR"
	ErrCodeNotFound       = "NOT_FOUND"
	ErrCodeConflict       = "CONFLICT"
	ErrCodeUnauthorized   = "UNAUTHORIZED"
)

const (
	MsgInvalidRequestBody  = "Invalid request body"
	MsgInternalServerError = "An internal server error occurred"
	MsgNotFound            = "Resource not found"
	MsgConflict            = "Conflict"
	MsgUnauthorized        = "Unauthorized"
)
```

- [ ] **Step 6: Remove the placeholder and run tests**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
rm -f pkg/http/.gitkeep
go test ./pkg/http/... -v
```
Expected: both tests PASS.

- [ ] **Step 7: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/http bot/go.mod bot/go.sum
git commit -m "feat(bot): add pkg/http net/http response helpers"
```

---

## Task 9: `pkg/tgbot` (Telegram wrapper)

**Files:**
- Create: `bot/pkg/tgbot/{bot,context,router,middleware}.go`, `bot/pkg/tgbot/router_test.go`
- Delete: `bot/pkg/tgbot/.gitkeep`

- [ ] **Step 1: Add the Telegram library**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go get github.com/go-telegram-bot-api/telegram-bot-api/v5@latest
```

- [ ] **Step 2: Write `bot/pkg/tgbot/context.go`**

```go
// Package tgbot wraps go-telegram-bot-api with a routing + context layer.
package tgbot

import (
	"context"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Context carries the active update, the bot client, and request-scoped context.
type Context struct {
	ctx    context.Context
	api    *tgbotapi.BotAPI
	Update tgbotapi.Update
}

func newContext(ctx context.Context, api *tgbotapi.BotAPI, upd tgbotapi.Update) *Context {
	return &Context{ctx: ctx, api: api, Update: upd}
}

// Ctx returns the request-scoped context (cancelled on shutdown).
func (c *Context) Ctx() context.Context { return c.ctx }

// API exposes the raw client for things the wrapper doesn't cover.
func (c *Context) API() *tgbotapi.BotAPI { return c.api }

// UserID returns the sending user's Telegram ID, or 0 if unknown.
func (c *Context) UserID() int64 {
	if u := c.Update.SentFrom(); u != nil {
		return u.ID
	}
	return 0
}

// ChatID returns the chat ID of the update, or 0 if unknown.
func (c *Context) ChatID() int64 {
	if ch := c.Update.FromChat(); ch != nil {
		return ch.ID
	}
	return 0
}

// Reply sends a plain-text reply to the originating chat.
func (c *Context) Reply(text string) error {
	if c.ChatID() == 0 {
		return nil
	}
	msg := tgbotapi.NewMessage(c.ChatID(), text)
	_, err := c.api.Send(msg)
	return err
}

// ReplyMarkdown sends a Markdown-formatted reply to the originating chat.
func (c *Context) ReplyMarkdown(text string) error {
	if c.ChatID() == 0 {
		return nil
	}
	msg := tgbotapi.NewMessage(c.ChatID(), text)
	msg.ParseMode = tgbotapi.ModeMarkdownV2
	_, err := c.api.Send(msg)
	return err
}
```

- [ ] **Step 3: Write `bot/pkg/tgbot/middleware.go`**

```go
package tgbot

// HandlerFunc handles a single update.
type HandlerFunc func(*Context) error

// Middleware wraps a HandlerFunc with cross-cutting behavior.
type Middleware func(HandlerFunc) HandlerFunc

// Chain composes middlewares right-to-left around h (outermost first).
func Chain(h HandlerFunc, mws ...Middleware) HandlerFunc {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
```

- [ ] **Step 4: Write the router test (TDD)**

Create `bot/pkg/tgbot/router_test.go`:
```go
package tgbot

import (
	"context"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

func TestRouter_DispatchesCommand(t *testing.T) {
	r := NewRouter()
	called := ""
	r.Handle("start", func(c *Context) error { called = "start"; return nil })

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{
			Text:     "/start",
			Entities: []tgbotapi.MessageEntity{{Type: "bot_command", Offset: 0, Length: 6}},
			Chat:     &tgbotapi.Chat{ID: 1},
		},
	}
	c := newContext(context.Background(), nil, upd)

	require.NoError(t, r.Dispatch(c))
	require.Equal(t, "start", called)
}

func TestRouter_DispatchesCallback(t *testing.T) {
	r := NewRouter()
	called := ""
	r.HandleCallback("yes", func(c *Context) error { called = "yes"; return nil })

	upd := tgbotapi.Update{
		CallbackQuery: &tgbotapi.CallbackQuery{Data: "yes"},
	}
	c := newContext(context.Background(), nil, upd)

	require.NoError(t, r.Dispatch(c))
	require.Equal(t, "yes", called)
}

func TestRouter_FallbackForUnknown(t *testing.T) {
	r := NewRouter()
	called := false
	r.HandleDefault(func(c *Context) error { called = true; return nil })

	upd := tgbotapi.Update{
		Message: &tgbotapi.Message{Text: "hi", Chat: &tgbotapi.Chat{ID: 1}},
	}
	c := newContext(context.Background(), nil, upd)

	require.NoError(t, r.Dispatch(c))
	require.True(t, called)
}
```

- [ ] **Step 5: Run the router tests, watch them fail**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go test ./pkg/tgbot/... -v
```
Expected: compile failure — `NewRouter`, `Router.Handle`, etc. undefined.

- [ ] **Step 6: Write `bot/pkg/tgbot/router.go`**

```go
package tgbot

// Router dispatches updates to registered handlers.
type Router struct {
	commands  map[string]HandlerFunc
	callbacks map[string]HandlerFunc
	fallback  HandlerFunc
}

func NewRouter() *Router {
	return &Router{
		commands:  map[string]HandlerFunc{},
		callbacks: map[string]HandlerFunc{},
	}
}

// Handle registers h for `/command` (without the slash).
func (r *Router) Handle(command string, h HandlerFunc) {
	r.commands[command] = h
}

// HandleCallback registers h for an exact callback_query data match.
func (r *Router) HandleCallback(data string, h HandlerFunc) {
	r.callbacks[data] = h
}

// HandleDefault registers a fallback for unmatched updates.
func (r *Router) HandleDefault(h HandlerFunc) {
	r.fallback = h
}

// Dispatch looks at the update and invokes the matching handler.
func (r *Router) Dispatch(c *Context) error {
	if c.Update.Message != nil && c.Update.Message.IsCommand() {
		if h, ok := r.commands[c.Update.Message.Command()]; ok {
			return h(c)
		}
	}
	if c.Update.CallbackQuery != nil {
		if h, ok := r.callbacks[c.Update.CallbackQuery.Data]; ok {
			return h(c)
		}
	}
	if r.fallback != nil {
		return r.fallback(c)
	}
	return nil
}
```

- [ ] **Step 7: Run the tests, watch them pass**

```bash
go test ./pkg/tgbot/... -v
```
Expected: 3 tests PASS.

- [ ] **Step 8: Write `bot/pkg/tgbot/bot.go`**

```go
package tgbot

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Bot wraps tgbotapi.BotAPI with a Router and middleware chain.
type Bot struct {
	api    *tgbotapi.BotAPI
	router *Router
	mws    []Middleware
}

// New connects to Telegram and returns a configured Bot.
func New(token string) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("tgbot: empty token")
	}
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("tgbot: connect: %w", err)
	}
	return &Bot{api: api, router: NewRouter()}, nil
}

// Use appends middlewares (outer-most first).
func (b *Bot) Use(mws ...Middleware) { b.mws = append(b.mws, mws...) }

// Router exposes the internal router for handler registration.
func (b *Bot) Router() *Router { return b.router }

// API exposes the underlying client.
func (b *Bot) API() *tgbotapi.BotAPI { return b.api }

// Run starts the long-poll loop. Returns when ctx is cancelled.
func (b *Bot) Run(ctx context.Context) error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	handler := Chain(b.router.Dispatch, b.mws...)

	for {
		select {
		case <-ctx.Done():
			b.api.StopReceivingUpdates()
			return ctx.Err()
		case upd, ok := <-updates:
			if !ok {
				return nil
			}
			c := newContext(ctx, b.api, upd)
			_ = handler(c) // logging middleware is responsible for surfacing errors
		}
	}
}
```

- [ ] **Step 9: Remove placeholder, ensure full package builds and tests pass**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
rm -f pkg/tgbot/.gitkeep
go build ./...
go test ./pkg/tgbot/... -v
```
Expected: build OK; 3 tests PASS.

- [ ] **Step 10: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/tgbot bot/go.mod bot/go.sum
git commit -m "feat(bot): add pkg/tgbot router, context, and bot"
```

---

## Task 10: `internal/middleware/logging.go`

**Files:**
- Create: `bot/internal/middleware/logging.go`
- Delete: `bot/internal/middleware/.gitkeep`

- [ ] **Step 1: Write `bot/internal/middleware/logging.go`**

```go
// Package middleware provides cross-cutting concerns for both HTTP and Telegram handlers.
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/<OWNER>/rollton/bot/pkg/tgbot"
)

// Telegram returns a tgbot.Middleware that logs each handled update.
func Telegram(log *slog.Logger) tgbot.Middleware {
	return func(next tgbot.HandlerFunc) tgbot.HandlerFunc {
		return func(c *tgbot.Context) error {
			start := time.Now()
			err := next(c)
			attrs := []any{
				"update_id", c.Update.UpdateID,
				"user_id", c.UserID(),
				"chat_id", c.ChatID(),
				"duration_ms", time.Since(start).Milliseconds(),
			}
			if err != nil {
				attrs = append(attrs, "err", err.Error())
				log.Error("telegram_update_failed", attrs...)
			} else {
				log.Info("telegram_update_handled", attrs...)
			}
			return err
		}
	}
}

// HTTP returns a net/http middleware that logs every request.
func HTTP(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: 200}
			next.ServeHTTP(sw, r)
			log.Info("http_request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
```

- [ ] **Step 2: Verify it builds**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
rm -f internal/middleware/.gitkeep
go build ./internal/middleware/...
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/middleware
git commit -m "feat(bot): add logging middleware for telegram and http"
```

---

## Task 11: `internal/config/config.go`

**Files:**
- Create: `bot/internal/config/config.go`, `bot/internal/config/config_test.go`
- Delete: `bot/internal/config/.gitkeep`

- [ ] **Step 1: Add godotenv**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go get github.com/joho/godotenv@latest
```

- [ ] **Step 2: Write the failing test**

Create `bot/internal/config/config_test.go`:
```go
package config_test

import (
	"testing"

	"github.com/<OWNER>/rollton/bot/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoad_FromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://u:p@localhost:5432/db?sslmode=disable")
	t.Setenv("HTTP_PORT", "8081")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "text")
	t.Setenv("TELEGRAM_TOKEN", "abc123")

	cfg, err := config.Load()
	require.NoError(t, err)

	require.Equal(t, "postgres://u:p@localhost:5432/db?sslmode=disable", cfg.DB.URL)
	require.Equal(t, 8081, cfg.HTTP.Port)
	require.Equal(t, "debug", cfg.Log.Level)
	require.Equal(t, "text", cfg.Log.Format)
	require.Equal(t, "abc123", cfg.TelegramToken)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	_, err := config.Load()
	require.Error(t, err)
}

func TestLoad_MissingTelegramToken(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "")
	_, err := config.Load()
	require.Error(t, err)
}
```

- [ ] **Step 3: Run, watch it fail**

```bash
go test ./internal/config/... -v
```
Expected: compile error — `config.Load` undefined.

- [ ] **Step 4: Write `bot/internal/config/config.go`**

```go
// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	DB            DBConfig
	HTTP          HTTPConfig
	Log           LogConfig
	TelegramToken string
}

type DBConfig struct {
	URL string
}

type HTTPConfig struct {
	Port int
}

type LogConfig struct {
	Level  string
	Format string
}

// Load reads env vars (and a local `.env` if present in cwd).
// TELEGRAM_TOKEN is per-process: each cmd/bot-X sets it before calling Load
// (or relies on the runtime env). DATABASE_URL is shared across bots.
func Load() (Config, error) {
	_ = godotenv.Load() // ignore: missing .env is fine in prod

	cfg := Config{
		DB:            DBConfig{URL: os.Getenv("DATABASE_URL")},
		HTTP:          HTTPConfig{Port: getEnvInt("HTTP_PORT", 8080)},
		Log:           LogConfig{Level: getEnvStr("LOG_LEVEL", "info"), Format: getEnvStr("LOG_FORMAT", "json")},
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
	}

	if cfg.DB.URL == "" {
		return Config{}, errors.New("config: DATABASE_URL is required")
	}
	if cfg.TelegramToken == "" {
		return Config{}, errors.New("config: TELEGRAM_TOKEN is required")
	}
	return cfg, nil
}

func getEnvStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	if n <= 0 {
		return def
	}
	return n
}
```

- [ ] **Step 5: Run tests**

```bash
rm -f internal/config/.gitkeep
go test ./internal/config/... -v
```
Expected: 3 tests PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/config bot/go.mod bot/go.sum
git commit -m "feat(bot): add config loader with env validation"
```

---

## Task 12: First bot wiring — `<BOT_A>` end-to-end

**Files:**
- Create: `bot/internal/bots/<BOT_A_PKG>/wire.go`
- Create: `bot/internal/bots/<BOT_A_PKG>/handlers/telegram/start_handler.go`
- Create: `bot/internal/bots/<BOT_A_PKG>/handlers/telegram/test/start_test.go`
- Create: `bot/internal/bots/<BOT_A_PKG>/handlers/http/healthz_handler.go`
- Modify: `bot/cmd/<BOT_A>/main.go` (replace placeholder)
- Delete: `.gitkeep` files under those dirs

- [ ] **Step 1: Add errgroup**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go get golang.org/x/sync/errgroup@latest
```

- [ ] **Step 2: Write the start-handler test (TDD)**

Create `bot/internal/bots/<BOT_A_PKG>/handlers/telegram/test/start_test.go`:
```go
package test

import (
	"testing"

	tgh "github.com/<OWNER>/rollton/bot/internal/bots/<BOT_A_PKG>/handlers/telegram"
	"github.com/stretchr/testify/require"
)

func TestStartHandler_ReplyText(t *testing.T) {
	h := tgh.NewStartHandler()
	require.Contains(t, h.ReplyText(), "Welcome")
}
```

- [ ] **Step 3: Run, watch it fail**

```bash
go test ./internal/bots/<BOT_A_PKG>/handlers/telegram/test/... -v
```
Expected: compile error — package empty.

- [ ] **Step 4: Write `start_handler.go`**

Create `bot/internal/bots/<BOT_A_PKG>/handlers/telegram/start_handler.go`:
```go
// Package telegram contains <BOT_A_PKG>'s Telegram update handlers.
package telegram

import (
	"github.com/<OWNER>/rollton/bot/pkg/tgbot"
)

type StartHandler struct{}

func NewStartHandler() *StartHandler { return &StartHandler{} }

// ReplyText is the exact message /start sends. Extracted so it's testable
// without a real Telegram API.
func (h *StartHandler) ReplyText() string {
	return "Welcome to <BOT_A>. This is a scaffold reply — no behaviour yet."
}

// Handle wires the handler into the tgbot router.
func (h *StartHandler) Handle(c *tgbot.Context) error {
	return c.Reply(h.ReplyText())
}
```

- [ ] **Step 5: Run, watch it pass**

```bash
go test ./internal/bots/<BOT_A_PKG>/handlers/telegram/test/... -v
```
Expected: 1 test PASS.

- [ ] **Step 6: Write the healthz handler**

Create `bot/internal/bots/<BOT_A_PKG>/handlers/http/healthz_handler.go`:
```go
// Package http contains <BOT_A_PKG>'s HTTP handlers.
package http

import (
	"net/http"

	httpx "github.com/<OWNER>/rollton/bot/pkg/http"
)

type HealthzHandler struct{}

func NewHealthzHandler() *HealthzHandler { return &HealthzHandler{} }

func (h *HealthzHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	httpx.WriteSuccess(w, map[string]string{"status": "ok", "bot": "<BOT_A>"})
}
```

- [ ] **Step 7: Write `wire.go`**

Create `bot/internal/bots/<BOT_A_PKG>/wire.go`:
```go
// Package <BOT_A_PKG> composes the <BOT_A> bot from its handlers and shared deps.
package <BOT_A_PKG>

import (
	"context"
	"fmt"
	"log/slog"
	httpstd "net/http"
	"time"

	httph "github.com/<OWNER>/rollton/bot/internal/bots/<BOT_A_PKG>/handlers/http"
	tgh "github.com/<OWNER>/rollton/bot/internal/bots/<BOT_A_PKG>/handlers/telegram"
	"github.com/<OWNER>/rollton/bot/internal/config"
	"github.com/<OWNER>/rollton/bot/internal/middleware"
	"github.com/<OWNER>/rollton/bot/pkg/tgbot"
	"golang.org/x/sync/errgroup"
)

type Deps struct {
	Cfg config.Config
	Log *slog.Logger
}

type App struct {
	deps Deps
	bot  *tgbot.Bot
	mux  *httpstd.ServeMux
}

func NewApp(deps Deps) (*App, error) {
	b, err := tgbot.New(deps.Cfg.TelegramToken)
	if err != nil {
		return nil, fmt.Errorf("<BOT_A_PKG>: %w", err)
	}
	b.Use(middleware.Telegram(deps.Log))

	start := tgh.NewStartHandler()
	b.Router().Handle("start", start.Handle)

	mux := httpstd.NewServeMux()
	mux.Handle("/healthz", httph.NewHealthzHandler())

	return &App{deps: deps, bot: b, mux: mux}, nil
}

// Run starts long-poll + HTTP. Returns when ctx is cancelled or either exits with error.
func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	srv := &httpstd.Server{
		Addr:              fmt.Sprintf(":%d", a.deps.Cfg.HTTP.Port),
		Handler:           middleware.HTTP(a.deps.Log)(a.mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	g.Go(func() error {
		a.deps.Log.Info("http_listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != httpstd.ErrServerClosed {
			return err
		}
		return nil
	})

	g.Go(func() error {
		a.deps.Log.Info("telegram_polling")
		err := a.bot.Run(gctx)
		if err != nil && err != context.Canceled {
			return err
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	return g.Wait()
}
```

- [ ] **Step 8: Replace `cmd/<BOT_A>/main.go`**

Overwrite `bot/cmd/<BOT_A>/main.go`:
```go
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	bota "github.com/<OWNER>/rollton/bot/internal/bots/<BOT_A_PKG>"
	"github.com/<OWNER>/rollton/bot/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config_load_failed", "err", err)
		os.Exit(1)
	}

	log := newLogger(cfg.Log.Format, cfg.Log.Level).With("bot", "<BOT_A>")

	app, err := bota.NewApp(bota.Deps{Cfg: cfg, Log: log})
	if err != nil {
		log.Error("app_init_failed", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx); err != nil && err != context.Canceled {
		log.Error("app_run_failed", "err", err)
		os.Exit(1)
	}
}

func newLogger(format, level string) *slog.Logger {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if format == "text" {
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, opts))
}
```

- [ ] **Step 9: Remove placeholders and build**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
find internal/bots/<BOT_A_PKG> -name .gitkeep -delete
go build ./...
go test ./... -short
```
Expected: build OK; all tests PASS.

- [ ] **Step 10: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/bots/<BOT_A_PKG> bot/cmd/<BOT_A> bot/go.mod bot/go.sum
git commit -m "feat(bot): wire up <BOT_A> with /start and /healthz"
```

---

## Task 13: Second bot — `<BOT_B>`

`<BOT_B>` is structurally identical to `<BOT_A>` at this point — same `wire.go`, same `start_handler.go`, same `healthz_handler.go`, just renamed.

**Files:**
- Create: `bot/internal/bots/<BOT_B_PKG>/wire.go`
- Create: `bot/internal/bots/<BOT_B_PKG>/handlers/telegram/start_handler.go`
- Create: `bot/internal/bots/<BOT_B_PKG>/handlers/telegram/test/start_test.go`
- Create: `bot/internal/bots/<BOT_B_PKG>/handlers/http/healthz_handler.go`
- Modify: `bot/cmd/<BOT_B>/main.go` (replace placeholder if it exists, otherwise create)

- [ ] **Step 1: Mirror the files via copy + rename**

From `rollton/`:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot

# package files
cp internal/bots/<BOT_A_PKG>/wire.go                                 internal/bots/<BOT_B_PKG>/wire.go
cp internal/bots/<BOT_A_PKG>/handlers/telegram/start_handler.go      internal/bots/<BOT_B_PKG>/handlers/telegram/start_handler.go
cp internal/bots/<BOT_A_PKG>/handlers/telegram/test/start_test.go    internal/bots/<BOT_B_PKG>/handlers/telegram/test/start_test.go
cp internal/bots/<BOT_A_PKG>/handlers/http/healthz_handler.go        internal/bots/<BOT_B_PKG>/handlers/http/healthz_handler.go

# rewrite references
LC_ALL=C find internal/bots/<BOT_B_PKG> cmd/<BOT_B> -type f -name '*.go' -print0 \
  | xargs -0 sed -i '' \
      -e 's|<BOT_A_PKG>|<BOT_B_PKG>|g' \
      -e 's|<BOT_A>|<BOT_B>|g'
```
(On Linux use `sed -i ''` without the trailing empty arg.)

- [ ] **Step 2: Create `cmd/<BOT_B>/main.go`**

If the file from Task 2's placeholder still exists, overwrite it. Otherwise create:
```bash
cp cmd/<BOT_A>/main.go cmd/<BOT_B>/main.go
LC_ALL=C sed -i '' \
  -e 's|<BOT_A_PKG>|<BOT_B_PKG>|g' \
  -e 's|<BOT_A>|<BOT_B>|g' \
  cmd/<BOT_B>/main.go
```

- [ ] **Step 3: Remove placeholders, build, test**

```bash
find internal/bots/<BOT_B_PKG> cmd/<BOT_B> -name .gitkeep -delete
go build ./...
go test ./... -short
```
Expected: build OK; tests for both bots PASS.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/bots/<BOT_B_PKG> bot/cmd/<BOT_B>
git commit -m "feat(bot): wire up <BOT_B> mirroring <BOT_A>"
```

---

## Task 14: End-to-end verification (no commit)

This task validates everything assembled so far. Nothing new is written; everything must pass.

- [ ] **Step 1: Clean build**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
make tidy
make build
ls bin/
```
Expected: `bin/<BOT_A>` and `bin/<BOT_B>` are present and executable.

- [ ] **Step 2: Compose postgres up**

```bash
make up
docker compose ps
```
Expected: postgres is healthy.

- [ ] **Step 3: Migration tooling is wired (no migrations expected)**

```bash
set -a; source .env; set +a
make migrate-status
```
Expected: exits 0, prints empty migration list.

- [ ] **Step 4: sqlc no-op**

```bash
make sqlc-gen
```
Expected: exits 0, no files generated.

- [ ] **Step 5: Run unit tests**

```bash
make test
```
Expected: all PASS.

- [ ] **Step 6: Lint passes**

```bash
make lint
```
Expected: no output, exits 0.

- [ ] **Step 7: Start `<BOT_A>` and hit healthz**

Requires a valid Telegram token in `.env` for `TELEGRAM_TOKEN`. If you don't have one yet, use a throwaway token from BotFather just for verification.

In one terminal:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
export TELEGRAM_TOKEN=$TOKEN_<BOT_A_UPPER>
make run BOT=<BOT_A>
```
In another terminal:
```bash
curl -s http://localhost:8080/healthz | python3 -m json.tool
```
Expected:
```json
{"success": true, "data": {"bot": "<BOT_A>", "status": "ok"}}
```
Open Telegram, message `/start` to the bot — expect the static reply. `Ctrl-C` the running process; it should log shutdown and exit cleanly within a few seconds.

- [ ] **Step 8: Tear down compose**

```bash
make down
```

- [ ] **Step 9: Final check — clean tree**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git status
```
Expected: clean tree. Optional final commit message tagging the scaffold:
```bash
git tag bot-scaffold-v0
```

---

## Out-of-scope (intentional)

- No domain code, no migrations, no sqlc queries, no DB adapters. Phase 8 of the spec (first real domain) begins after this plan.
- No CI workflow. Add when the first real feature lands.
- No Swagger. Add when HTTP REST grows.
- No metrics or tracing — `slog` is the only observability.
- No webhook receiver — long-poll only.
- `infra/` stays as a placeholder README.
