# Rollton — `bot/` structure & migration plan

**Date:** 2026-06-12
**Status:** Approved (design phase)
**Scope:** Scaffold the `bot/` directory of the `rollton` monorepo. `infra/` (Terraform) is out of scope.

## 1. Context

`rollton` is a monorepo:

- `bot/` — Telegram bots written in Go. Multiple bots, shared internals, one Go module, one Postgres database.
- `infra/` — Terraform for AWS resources. Placeholder for now.

The reference architecture is `~/Desktop/projects.pet/viza_assigment` (Go hex-arch HTTP service: `cmd/internal/pkg/db/docs`, Atlas + sqlc + PostgreSQL + Docker). This spec mirrors that structure with two deliberate changes:

1. **Atlas → goose** for migrations.
2. **HTTP handlers + Telegram update handlers** instead of HTTP-only. HTTP is kept (healthz now, REST later); Telegram is the primary update channel via long-poll.

Plus one organizational change: a per-bot vertical slice under `internal/bots/<name>/` instead of viza_assigment's flat `internal/handlers/`.

## 2. Decisions

| Decision | Choice | Notes |
|---|---|---|
| Bots in repo | Multiple, shared internals (Option B from brainstorming) | One Go module, multiple `cmd/bot-X/main.go` binaries. |
| Project layout | Per-bot vertical slice (Option 2) | `internal/bots/<name>/` owns bot-specific handlers + services; shared code in `internal/core/`. |
| Telegram library | `github.com/go-telegram-bot-api/telegram-bot-api/v5` | Mature, popular, simple. Wrapped by `pkg/tgbot/` so swapping later costs less. |
| Update delivery | Long-poll (primary). HTTP server kept for healthz and future REST. | Webhook can be added later without restructure. |
| Database | One shared Postgres DB, one shared schema | Bots distinguished by `bot_id` column where needed. |
| Migrations | `goose` (replaces viza_assigment's Atlas) | Library + CLI; programmatic migration in tests. |
| Code generation | `sqlc` (kept from viza_assigment) | Single config, output to `pkg/sqlc/postgres/`. |
| Logging | `slog` (stdlib) | Structured, context-aware. |
| HTTP router | `net/http` + viza_assigment's `pkg/http` helpers | Add `chi` only if/when REST grows. |
| Tests | `testify/require` for unit; `testcontainers-go` for integration | Mirrors viza_assigment's split: production code clean, tests in sibling `test/` dirs. |
| Vendoring | Off (module cache only) | Deviates from viza_assigment, which vendors. |
| CI | None yet | Add when first bot has real domain code. |
| Swagger | Not now | Re-introduce when HTTP REST appears. |

## 3. Target directory layout

```
rollton/
├── README.md
├── Makefile                       # delegates to bot/ and infra/
├── .gitignore
├── bot/
│   ├── cmd/
│   │   ├── bot-a/main.go
│   │   └── bot-b/main.go
│   ├── internal/
│   │   ├── bots/
│   │   │   ├── bot_a/
│   │   │   │   ├── handlers/
│   │   │   │   │   ├── telegram/
│   │   │   │   │   │   └── test/
│   │   │   │   │   └── http/
│   │   │   │   │       └── test/
│   │   │   │   ├── services/
│   │   │   │   │   └── test/
│   │   │   │   └── wire.go
│   │   │   └── bot_b/             # same shape
│   │   ├── core/
│   │   │   ├── domain/
│   │   │   │   └── test/
│   │   │   ├── ports/
│   │   │   └── services/
│   │   │       └── test/
│   │   ├── config/
│   │   ├── middleware/
│   │   └── testutil/              # NewTestDB(t), shared test helpers (unexported)
│   ├── pkg/
│   │   ├── http/                  # response + errors helpers
│   │   ├── tgbot/                 # bot, router, context, middleware
│   │   └── sqlc/postgres/         # generated + adapter types
│   ├── db/
│   │   ├── migrations/            # goose *.sql
│   │   ├── queries/               # sqlc input
│   │   └── schema/schema.sql      # regenerated snapshot
│   ├── docs/
│   ├── Dockerfile
│   ├── docker-compose.yml
│   ├── Makefile
│   ├── sqlc.yaml
│   ├── .air.toml
│   ├── .env.example
│   └── go.mod                     # github.com/<owner>/rollton/bot
└── infra/
    └── README.md                  # placeholder
```

## 4. Component responsibilities

### 4.1 `cmd/bot-<name>/main.go`
Composition root only. Loads config, opens `*sql.DB`, builds sqlc `Queries`, wraps in adapters, constructs services, calls `internal/bots/<name>.NewApp(deps)`, handles signals, runs `app.Run(ctx)`.

### 4.2 `internal/config/`
`Load() (Config, error)` reads env (godotenv in dev). `Config{DB, HTTP, Log, Bots map[string]BotConfig}`. Each `cmd/bot-X` reads its slice by name (`cfg.Bots["bot-a"]`).

### 4.3 `internal/core/`

- `domain/` — pure types and invariants. No DB, no Telegram imports. Tests in `domain/test/`.
- `ports/` — repository and service interfaces. Defines the dependency-inversion boundary.
- `services/` — implementations of cross-bot use cases. Depend on ports only. Tests in `services/test/` with fake repos.

### 4.4 `internal/bots/<name>/`

- `services/` — bot-specific use cases. Same pattern as `core/services/`.
- `handlers/telegram/` — one file per command/feature (`start_handler.go`, `callback_handler.go`, …). Each handler is a method on a struct holding services.
- `handlers/http/` — `healthz_handler.go` initially; future REST endpoints.
- `wire.go` — `NewApp(deps Deps) *App` builds router, registers handlers, returns `App` with `Run(ctx) error`. Only place handlers are instantiated.

### 4.5 `internal/middleware/`
`LoggingMiddleware` (telegram + HTTP variants), `RecoverMiddleware`. Cross-cutting, lives outside `bots/`.

### 4.6 `pkg/tgbot/`

- `bot.go` — `New(token, opts...)`, `Run(ctx)` long-poll loop.
- `router.go` — `Handle(cmd, h)`, `HandleCallback`, `HandleMessage(pattern)`, `Dispatch(update)`.
- `context.go` — wraps `*tgbotapi.Update` with `Reply`, `ReplyMarkdown`, `UserID`, `ChatID`, `Args`.
- `middleware.go` — `Middleware func(HandlerFunc) HandlerFunc`, `Chain(...)`.

### 4.7 `pkg/sqlc/postgres/`
sqlc-generated code from `db/queries/*.sql`. Adapter types (e.g., `UserRepoPg`) wrap `*Queries` and satisfy `core/ports` interfaces, translating between sqlc rows and `domain.*` types. The **only** place sqlc types appear.

### 4.8 `pkg/http/`
`Response`, `Error` helpers copied verbatim from viza_assigment.

### 4.9 `db/`

- `migrations/` — goose-annotated `YYYYMMDDHHMMSS_name.sql`.
- `queries/` — sqlc input `.sql` files.
- `schema/schema.sql` — canonical snapshot regenerated from migrations by `make schema-dump`.

### 4.10 Dependency direction (strict)

```
handlers → services → ports ← adapters (pkg/sqlc/postgres)
                       ↑
                     domain
```

`cmd/*` and `wire.go` are the only places that know all three sides.

## 5. Data flow

### 5.1 Startup (per bot)

1. `config.Load()` → `Config`.
2. Open `*sql.DB` (`pgx/v5/stdlib`), ping, set pool limits.
3. `queries := postgres.New(db)`.
4. Wrap in adapters (`userRepo := postgres.NewUserRepoPg(queries)`).
5. Build shared services (`userSvc := services.NewUserService(userRepo, ...)`).
6. `app := bot_a.NewApp(bot_a.Deps{Cfg: cfg.Bots["bot-a"], UserSvc: userSvc, ...})`.
7. `app.Run(ctx)` runs (a) `pkg/tgbot.Bot` long-poll, (b) `net/http.Server` for healthz, both under `errgroup` honoring `ctx.Done()`.

### 5.2 Telegram update path

```
Telegram API → pkg/tgbot.Bot (long-poll) → middleware chain (recover → logging → …)
  → pkg/tgbot.Router.Dispatch (command / callback_data / message-pattern match)
  → internal/bots/<name>/handlers/telegram.XHandler.Handle(ctx *tgbot.Context)
  → service call (core or bot-specific)
  → ports.XRepository.Method
  → pkg/sqlc/postgres.XRepoPg (translates ports ↔ sqlc)
  → sqlc query → Postgres
  → result bubbles back as domain.X → handler builds reply → ctx.Reply(...)
```

### 5.3 HTTP path
Same shape: `net/http` mux → middleware → handler struct method → service → repo. Same services and adapters as the Telegram path.

### 5.4 Shutdown

- `cmd` catches `SIGINT`/`SIGTERM`, cancels root `ctx`.
- `errgroup` waits for `tgbot.Bot` (stops poller, closes update channel) and `http.Server.Shutdown(ctx)`.
- `db.Close()` last.

### 5.5 Multi-bot operation
Each `cmd/bot-X` is its own binary and OS process. Bots share only Postgres. Compose runs them as separate services; in prod each is its own deployment.

## 6. Error handling

- **Sentinel errors per layer.** `core/domain` defines `ErrUserNotFound`, `ErrInvalidInput`, `ErrConflict`, …. Adapters translate driver errors into sentinels (`pgx.ErrNoRows` → `ErrUserNotFound`, unique violation → `ErrConflict`).
- **Services** pass through or wrap with `fmt.Errorf("...: %w", err)`.
- **Handlers** map errors to user-visible output: Telegram handlers `errors.Is` and reply in plain language; HTTP handlers use `pkg/http.Error(w, err)` which maps sentinels to status codes (kept identical to viza_assigment).
- **Recover middleware** logs the stack, replies with a generic message, never crashes the process for a single bad update or request.
- **Structured logging** via `slog`. One logger built in `cmd`, passed via context. Fields: `level, msg, bot, update_id|request_id, user_id, err`. No `fmt.Println`.
- **Context everywhere.** All service + repo methods take `ctx`. Cancellation on shutdown propagates to in-flight DB queries.
- **No panics for control flow.** Genuine startup misconfig: `log.Fatal` in `cmd`. Runtime panics: caught by recover middleware.

## 7. Testing

| Location | Scope | Style |
|---|---|---|
| `internal/core/domain/test/` | Invariants, constructors | Pure unit, no mocks |
| `internal/core/services/test/` | Cross-bot use cases | In-memory fake repos satisfying `ports.*Repository` |
| `internal/bots/<name>/services/test/` | Bot-specific use cases | Same as above |
| `internal/bots/<name>/handlers/test/` | Handler dispatch + reply text/markup | Fake `tgbot.Context` capturing replies; fake services |
| `pkg/sqlc/postgres/test/` | sqlc queries + adapter translation | `testcontainers-go` + Postgres; goose runs migrations before each suite |
| `pkg/tgbot/` | Router matching, middleware chain | Internal package tests |

**Tooling:**
- `github.com/stretchr/testify/require` for assertions.
- `github.com/testcontainers/testcontainers-go` + `postgres` module for integration DB.
- `github.com/pressly/goose/v3` library used in tests to run migrations against the testcontainer.
- `internal/testutil/` (unexported) provides `NewTestDB(t)` helper.

**Make targets:** `make test` (unit only), `make test-integration` (testcontainers), `make coverage`.

## 8. Migration plan (scaffolding steps)

### Phase 0 — Repo bootstrap

1. `git init` in `rollton/`.
2. Root `.gitignore`: Go binaries, `.env`, `*.test`, `coverage.out`, `bot/bin/`, `.idea/`.
3. Root `README.md`: monorepo overview, pointers to `bot/` and `infra/`.
4. Root `Makefile`: delegates `make -C bot <target>`, `make -C infra <target>`.
5. `infra/README.md`: placeholder ("terraform — TBD").

### Phase 1 — `bot/` module init

1. `cd bot && go mod init github.com/<owner>/rollton/bot` (owner TBD at execution time).
2. Copy & adapt from viza_assigment:
   - `Dockerfile`: multi-stage; `ARG BOT`; `go build ./cmd/$BOT`.
   - `docker-compose.yml`: `postgres` service + one entry per bot, env-file driven.
   - `Makefile`: rewrite migration targets for goose; keep test/lint/coverage targets.
   - `.air.toml`: parametrize binary path via `BOT` env (or one `.air-<bot>.toml` each).
   - `.env.example`: DB + per-bot `TOKEN_BOT_A`, `TOKEN_BOT_B` vars.
3. `bot/.vscode/` (selective): `settings.json`, `launch.json` per bot.

### Phase 2 — Layout skeleton

Create all directories with `.gitkeep` placeholders:

```
bot/cmd/bot-a/        bot/cmd/bot-b/
bot/internal/bots/bot_a/{handlers/telegram,handlers/http,services}
bot/internal/bots/bot_a/handlers/{telegram,http}/test
bot/internal/bots/bot_a/services/test
bot/internal/bots/bot_b/{handlers/telegram,handlers/http,services}
bot/internal/bots/bot_b/handlers/{telegram,http}/test
bot/internal/bots/bot_b/services/test
bot/internal/core/{domain,ports,services}
bot/internal/core/{domain,services}/test
bot/internal/{config,middleware,testutil}
bot/pkg/{http,tgbot,sqlc/postgres}
bot/pkg/sqlc/postgres/test
bot/db/{migrations,queries,schema}
bot/docs/
```

### Phase 3 — goose setup

1. Add library dep: `go get github.com/pressly/goose/v3`.
2. Install CLI: documented in `bot/README.md` (`go install github.com/pressly/goose/v3/cmd/goose@latest`) or use Go 1.24+ tool dependency.
3. `Makefile` migration targets:
   ```make
   GOOSE_DRIVER   ?= postgres
   GOOSE_DBSTRING ?= $(DATABASE_URL)
   GOOSE_DIR      ?= db/migrations

   migrate-new:    ; goose -dir $(GOOSE_DIR) create $(name) sql
   migrate-up:     ; goose -dir $(GOOSE_DIR) postgres "$(GOOSE_DBSTRING)" up
   migrate-down:   ; goose -dir $(GOOSE_DIR) postgres "$(GOOSE_DBSTRING)" down
   migrate-status: ; goose -dir $(GOOSE_DIR) postgres "$(GOOSE_DBSTRING)" status
   migrate-reset:  ; goose -dir $(GOOSE_DIR) postgres "$(GOOSE_DBSTRING)" reset
   ```
4. Naming: goose default `YYYYMMDDHHMMSS_name.sql` with `-- +goose Up` / `-- +goose Down` annotations.
5. `make schema-dump`: spins up throwaway Postgres container, runs `migrate-up`, `pg_dump --schema-only > db/schema/schema.sql`.
6. Programmatic migrations in tests via `goose.Up(db, "db/migrations")`.

### Phase 4 — sqlc setup

1. Copy & adapt `sqlc.yaml` from viza_assigment (engine: postgresql, schema: `db/schema/schema.sql`, queries: `db/queries/`, out: `pkg/sqlc/postgres`).
2. `make sqlc-gen` target.
3. No queries yet — first query lands with the first feature.

### Phase 5 — Shared packages

1. `pkg/http/{response,errors}.go` — copy from viza_assigment as-is.
2. `pkg/tgbot/` — write fresh:
   - `bot.go` — `New(token, opts...)`, `Run(ctx)`.
   - `router.go` — `Handle`, `HandleCallback`, `HandleMessage`, `Dispatch`.
   - `context.go` — `Context` wrapping `*tgbotapi.Update`.
   - `middleware.go` — `Middleware`, `Chain`.
3. `internal/middleware/logging.go` — `slog` based; provides both `tgbot.Middleware` and `func(http.Handler) http.Handler`.

### Phase 6 — Composition skeleton

1. `internal/config/config.go` — env loader, `Config` struct.
2. `internal/bots/bot_a/wire.go` — `Deps`, `App`, `NewApp(deps) *App`, `(*App).Run(ctx) error` using `errgroup`.
3. `cmd/bot-a/main.go` — composes everything, ~40 lines. Same for `bot-b`.
4. First handler: `internal/bots/bot_a/handlers/telegram/start_handler.go` — static reply to `/start`, proves end-to-end wiring without DB.

### Phase 7 — Verification

1. `make build` builds both bots.
2. `docker compose up postgres` brings up DB.
3. `make migrate-up` succeeds (no-op).
4. `make sqlc-gen` succeeds (no-op).
5. `go run ./cmd/bot-a` with `TOKEN_BOT_A` set: long-polls Telegram, replies to `/start`.
6. `make test` passes (stub tests only).
7. `curl localhost:8080/healthz` returns 200.

### Phase 8 — First real domain (out of scope for this spec; entry point)

1. Write first migration in `db/migrations/`.
2. `make schema-dump` regenerates `db/schema/schema.sql`.
3. Add first query in `db/queries/users.sql`; `make sqlc-gen`.
4. Define `domain.User`, `ports.UserRepository`, adapter, service, handler.
5. Repeat per feature.

## 9. Explicit non-goals

- No CI/CD pipeline yet.
- No Swagger / OpenAPI.
- No `vendor/` directory (deviates from viza_assigment).
- No metrics / tracing (logging-only for now).
- No webhook delivery (long-poll only).
- No `infra/` content beyond a placeholder README.

## 10. Open items to resolve at execution time

- Module path owner: `github.com/<owner>/rollton/bot` — needs the actual GitHub owner.
- Initial bot names: spec uses placeholders `bot-a` / `bot-b`. Real names go in at scaffolding time.
- Go version: pin in `go.mod` based on what's installed when scaffolding starts (target Go 1.22+ for `slog` stdlib).
- VS Code launch configs are optional — include only if convenient.
