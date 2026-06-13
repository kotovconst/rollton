# Telegram user auto-registration — design

**Date:** 2026-06-13
**Status:** Approved (design phase)
**Scope:** First real domain feature in `bot/`. When a Telegram user interacts with any rolltonchatbot binary for the first time, automatically register them in the database. All subsequent handlers receive a populated `*domain.User` from the request context.

## 1. Context

The bot scaffold so far (`bot/`) has the architecture in place but no domain logic — there are no migrations, no sqlc queries, no domain types, no repositories. This spec is Phase 8 from the bot scaffold spec ("First real domain"): the first end-to-end vertical slice.

Why start with user registration:
- It's a prerequisite for every other feature (settings, subscription, character chats) — they all assume a `user_id`.
- It exercises every layer: migration → sqlc query → domain → port → adapter → service → middleware → handler.
- It produces a stable foundation the mini-app's `/api/v1/me` endpoint will later sit on.

The same `users` table serves both `rolltonchatbot` (the launcher) and `admin` bots. Per-character bots (future) will also reuse it.

## 2. Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Registration trigger | Middleware on every update (Option B) | Sets `*domain.User` in context once; handlers don't repeat the lookup |
| Welcome message | Sent by `/start` handler only, not the middleware | Middleware is silent; commands are visible |
| Username/name freshness | Upsert on every update (`ON CONFLICT (telegram_id) DO UPDATE`) | Telegram username/first_name/last_name can change |
| Internal ID | UUID v4 | Stable, doesn't leak Telegram ID, fits future API contracts |
| Telegram ID storage | `BIGINT NOT NULL UNIQUE` | Telegram IDs can exceed int32; uniqueness enforces one-row-per-user |
| Cross-bot sharing | All bots share one `users` table | One user maps to one row regardless of which bot they talked to first |
| Updates to existing rows | Whitelist of columns (`username`, `first_name`, `last_name`, `language_code`, `is_premium`); never change `id`, `telegram_id`, `created_at` | Idempotency |
| Empty `SentFrom()` updates (e.g. channel posts) | Middleware passes through without DB call | Bot-only edge cases; nothing to register |

## 3. Database schema

First migration: `db/migrations/<timestamp>_create_users.sql` (goose-annotated).

```sql
-- +goose Up
CREATE TABLE users (
    id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id     BIGINT      NOT NULL UNIQUE,
    username        TEXT,
    first_name      TEXT        NOT NULL,
    last_name       TEXT,
    language_code   TEXT,
    is_premium      BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX users_telegram_id_idx ON users (telegram_id);

-- +goose Down
DROP TABLE users;
```

(`gen_random_uuid()` is built into Postgres 13+; no `uuid-ossp` extension needed for Postgres 16.)

After applying: `make schema-dump` regenerates `db/schema/schema.sql` so sqlc sees the new table.

## 4. sqlc queries

`db/queries/users.sql`:

```sql
-- name: GetUserByTelegramID :one
SELECT * FROM users WHERE telegram_id = $1;

-- name: UpsertUserFromTelegram :one
INSERT INTO users (telegram_id, username, first_name, last_name, language_code, is_premium)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (telegram_id) DO UPDATE SET
    username      = EXCLUDED.username,
    first_name    = EXCLUDED.first_name,
    last_name     = EXCLUDED.last_name,
    language_code = EXCLUDED.language_code,
    is_premium    = EXCLUDED.is_premium,
    updated_at    = NOW()
RETURNING *;
```

Two queries cover everything: a fast read path for the common case (user already exists) and an upsert for the cold path or when fields drift.

## 5. Layered components

### 5.1 `internal/core/domain/user.go`
Pure type — no DB, no Telegram imports.

```go
package domain

import (
    "time"
    "github.com/google/uuid"
)

type User struct {
    ID            uuid.UUID
    TelegramID    int64
    Username      string  // empty if not set in Telegram
    FirstName     string
    LastName      string  // empty if not set
    LanguageCode  string
    IsPremium     bool
    CreatedAt     time.Time
    UpdatedAt     time.Time
}

type TelegramUserInput struct {
    TelegramID    int64
    Username      string
    FirstName     string
    LastName      string
    LanguageCode  string
    IsPremium     bool
}
```

Sentinels:
```go
var ErrUserNotFound = errors.New("user not found")
```

### 5.2 `internal/core/ports/user_repository.go`
Interface — no implementations here.

```go
package ports

import (
    "context"
    "github.com/kotovconst/rollton/bot/internal/core/domain"
)

type UserRepository interface {
    GetByTelegramID(ctx context.Context, tgID int64) (domain.User, error)
    UpsertFromTelegram(ctx context.Context, input domain.TelegramUserInput) (domain.User, error)
}
```

### 5.3 `internal/core/services/user_service.go`

```go
type UserService struct {
    repo ports.UserRepository
}

func NewUserService(repo ports.UserRepository) *UserService { ... }

// EnsureRegistered returns the existing user if one exists, otherwise creates one.
// Fields that drift between Telegram updates (username, first_name, ...) are refreshed.
// Hot path: one SELECT. Cold path: one UPSERT.
func (s *UserService) EnsureRegistered(ctx context.Context, input domain.TelegramUserInput) (domain.User, error) {
    u, err := s.repo.GetByTelegramID(ctx, input.TelegramID)
    if err == nil && fieldsMatch(u, input) {
        return u, nil
    }
    return s.repo.UpsertFromTelegram(ctx, input)
}
```

`fieldsMatch` compares the Telegram-sourced fields between the stored row and the incoming update — avoids a write on every interaction.

### 5.4 `pkg/sqlc/postgres/user_repo.go`
Adapter — the only place where sqlc-generated types appear.

```go
type UserRepoPg struct {
    q *Queries
}

func NewUserRepoPg(q *Queries) *UserRepoPg { ... }

func (r *UserRepoPg) GetByTelegramID(ctx context.Context, tgID int64) (domain.User, error) {
    row, err := r.q.GetUserByTelegramID(ctx, tgID)
    if errors.Is(err, pgx.ErrNoRows) {
        return domain.User{}, domain.ErrUserNotFound
    }
    if err != nil {
        return domain.User{}, fmt.Errorf("user_repo: get: %w", err)
    }
    return toDomainUser(row), nil
}

func (r *UserRepoPg) UpsertFromTelegram(ctx context.Context, input domain.TelegramUserInput) (domain.User, error) {
    // construct sqlc param struct, call UpsertUserFromTelegram, translate result
}

func toDomainUser(row postgres.User) domain.User { ... }
```

### 5.5 `internal/middleware/auth.go`

The user is stored in the standard `context.Context` carried by `tgbot.Context`, using a private key in the middleware package. No changes to `pkg/tgbot` are required — this keeps the dependency direction clean (`pkg/tgbot` knows nothing about `domain.User`).

```go
type userCtxKey struct{}

// WithUser returns a new context carrying u.
func WithUser(ctx context.Context, u *domain.User) context.Context {
    return context.WithValue(ctx, userCtxKey{}, u)
}

// UserFromContext returns the registered user, or (nil, false) if no
// EnsureUserRegistered middleware ran or the registration failed soft.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
    u, ok := ctx.Value(userCtxKey{}).(*domain.User)
    return u, ok
}

// EnsureUserRegistered is a tgbot.Middleware that auto-registers the sender.
// If SentFrom() is nil (channel posts, edited-without-user updates, etc.) it
// passes through without DB calls.
func EnsureUserRegistered(svc *services.UserService, log *slog.Logger) tgbot.Middleware {
    return func(next tgbot.HandlerFunc) tgbot.HandlerFunc {
        return func(c *tgbot.Context) error {
            tg := c.Update.SentFrom()
            if tg == nil {
                return next(c)
            }
            input := domain.TelegramUserInput{
                TelegramID:   tg.ID,
                Username:     tg.UserName,
                FirstName:    tg.FirstName,
                LastName:     tg.LastName,
                LanguageCode: tg.LanguageCode,
                IsPremium:    tg.IsPremium,
            }
            u, err := svc.EnsureRegistered(c.Ctx(), input)
            if err != nil {
                log.Error("user_registration_failed",
                    "telegram_id", tg.ID, "err", err)
                return next(c) // continue with no user in context
            }
            // Mutate the tgbot.Context's ctx to carry the user.
            c.SetCtx(WithUser(c.Ctx(), &u))
            return next(c)
        }
    }
}
```

### 5.6 `pkg/tgbot/context.go` — one small addition

Add a single setter so middleware can replace the carried `context.Context`:

```go
// SetCtx replaces the request-scoped context.
func (c *Context) SetCtx(ctx context.Context) { c.ctx = ctx }
```

That's the only change to `pkg/tgbot`. No new fields, no `domain.User` import, no map of values.

### 5.7 `internal/bots/rolltonchatbot/wire.go`
- Construct `userRepo := postgres.NewUserRepoPg(queries)`.
- Construct `userSvc := services.NewUserService(userRepo)`.
- `b.Use(middleware.EnsureUserRegistered(userSvc, log))`.
- Same for `admin` bot's wire.go.

### 5.8 `/start` handler update
Reply now uses the user's first name:

```go
func (h *StartHandler) Handle(c *tgbot.Context) error {
    u, _ := middleware.UserFromContext(c.Ctx())
    name := "there"
    if u != nil { name = u.FirstName }
    return c.Reply(fmt.Sprintf("Hi %s, welcome to rolltonchatbot.", name))
}
```

## 6. Data flow

```
update arrives → tgbot.Bot dispatches
  → logging middleware
  → EnsureUserRegistered middleware
       │
       ▼
   SentFrom() == nil ? → call next(c) directly
       │
       ▼ (yes user)
   userService.EnsureRegistered(ctx, input)
       │
       ├── repo.GetByTelegramID(tgID)
       │     ├── found + fields match → return cached domain.User
       │     └── ErrUserNotFound or drift detected → repo.UpsertFromTelegram
       ↓
   put *domain.User in ctx
       ↓
   next(c) → router → start_handler / callback_handler / ...
       ↓
   handler reads UserFromContext(c.Ctx()) → personalized reply
```

## 7. Error handling

- **DB unreachable / connection error:** middleware logs `user_registration_failed` and passes `c` through to handlers *without* a user in ctx. Handlers fall back to a generic greeting. Better than blocking the bot on every update during a Postgres outage.
- **Unique constraint race (two simultaneous registers from the same `telegram_id`):** Postgres ON CONFLICT handles it idempotently; both updates return the same row.
- **Invalid Telegram payload** (`SentFrom().FirstName == ""`): Telegram guarantees `first_name` is present; defensive — service treats empty as `"user"` and proceeds. Never blocks.
- **Domain sentinel:** `domain.ErrUserNotFound` is only meaningful inside `EnsureRegistered`'s hot path. Callers don't see it (the upsert always returns a user).

## 8. Testing

| Layer | Test file | Style |
|---|---|---|
| Domain | `internal/core/domain/test/user_test.go` | Pure unit |
| Service (fake repo) | `internal/core/services/test/user_service_test.go` | Hot path (found + match) returns without write; cold path (missing) calls upsert; drift path triggers upsert |
| Middleware (fake service) | `internal/middleware/test/auth_test.go` | nil `SentFrom()` → passes through; service error → still calls next with no user; success → user in ctx |
| Adapter | `pkg/sqlc/postgres/test/user_repo_test.go` | `testcontainers-go` + goose migration; round-trip Get/Upsert |
| Handler | `internal/bots/rolltonchatbot/handlers/telegram/test/start_test.go` (update existing) | With user in ctx → personalized; without user → fallback greeting |

Testcontainers integration test is opt-in: `make test-integration`. Unit tests stay in `make test`.

## 9. Out-of-scope (deferred)

- `/api/v1/me` HTTP endpoint for the mini app (separate spec).
- TMA initData HMAC validation middleware for HTTP (separate spec).
- User profile editing (settings, language preference).
- Admin commands to inspect or modify users.
- Soft-delete or GDPR-style user deletion.
- Audit log of registration events.
- Per-bot "first seen" tracking (which bot registered them).
- Rate limiting on the upsert path.
- i18n / localized welcome messages.

## 10. Open items resolved at execution time

- Approach for carrying the user through the request: standard `context.WithValue` keyed on a sentinel type defined in `internal/middleware/auth.go`. `pkg/tgbot.Context` gets a one-line `SetCtx` setter and nothing else. (Decided in Section 5.5–5.6.)
- Initial migration timestamp (`make migrate-new name=create_users` decides this at execution).
- Whether `is_premium` defaults FALSE or NULL — defaults to FALSE per Section 3.
- Whether to keep `pgx.ErrNoRows` translation in the adapter or domain layer — adapter, per Section 5.4 (sqlc-emitted error stays inside sqlc package).
