# Character Chats Schema Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the database schema for characters, contexts, model configs, chats, conversation history, and the age-gate / active-chat additions to `users`. After this plan: 5 new tables + 3 new columns on `users` + 1 seed row. **Schema only — no service layer, no queries, no Go code beyond regenerating `schema.sql`.**

**Architecture:** Seven goose migrations applied in dependency order. Each migration is small and atomic, applied and verified against a live local Postgres before moving on. Schema snapshot regenerated at the end so future `sqlc generate` runs see the new tables.

**Tech Stack:** Postgres 16 (`gen_random_uuid()` built-in, no extensions needed), goose ≥ v3 (`-- +goose StatementBegin/End` annotations), the existing `bot/Makefile` migration targets, `psql` for verification.

**Reference spec:** `docs/superpowers/specs/2026-06-14-rollton-character-chats-schema-design.md`.

---

## Pre-flight

- Docker daemon running. (`docker version` returns non-error.)
- `make up` has been run; `rollton-postgres` container is `(healthy)`.
- `bot/.env` has a valid `DATABASE_URL` (already set up earlier in session).
- Existing migration `20260613082738_create_users.sql` is **applied** (not required for these new migrations to write, but required before applying any of them — without `users`, the chats FK fails).

If the existing users migration isn't applied yet:

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-up
make migrate-status   # confirm 20260613082738_create_users is applied
```

---

## File Structure

```
bot/db/
├── migrations/
│   ├── 20260613082738_create_users.sql              # EXISTING (already applied)
│   ├── <ts1>_create_model_configs.sql               # Task 1
│   ├── <ts2>_create_characters.sql                  # Task 2
│   ├── <ts3>_create_contexts.sql                    # Task 3
│   ├── <ts4>_create_chats.sql                       # Task 4
│   ├── <ts5>_create_tg_messages.sql                 # Task 5
│   ├── <ts6>_extend_users.sql                       # Task 6
│   └── <ts7>_seed_default_model_config.sql          # Task 7
└── schema/
    └── schema.sql                                    # MODIFIED in Task 8 (regenerated)
```

Goose generates timestamps at `make migrate-new` time; the order they're created in this plan is the order they apply in.

---

## Task 1: `model_configs`

**Files:**
- Create: `bot/db/migrations/<ts>_create_model_configs.sql`

- [ ] **Step 1: Generate the migration file**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-new name=create_model_configs
```
Expected: `Created new file: db/migrations/<timestamp>_create_model_configs.sql`. Note the timestamp.

- [ ] **Step 2: Replace the file contents**

Open `bot/db/migrations/<ts>_create_model_configs.sql` and replace entirely with:

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE model_configs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          TEXT        NOT NULL UNIQUE,
    display_name  TEXT        NOT NULL,
    provider      TEXT        NOT NULL,
    model         TEXT        NOT NULL,
    temperature   DOUBLE PRECISION NULL,
    top_p         DOUBLE PRECISION NULL,
    max_tokens    INTEGER     NULL,
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE model_configs;
-- +goose StatementEnd
```

- [ ] **Step 3: Apply**

```bash
make migrate-up
make migrate-status
```
Expected: the new migration shows as Applied.

- [ ] **Step 4: Verify table structure**

```bash
docker compose exec -T postgres psql -U rollton rollton -c '\d model_configs'
```
Expected: shows `id, slug, display_name, provider, model, temperature, top_p, max_tokens, is_active, created_at, updated_at` with the unique constraint on `slug` and the PK on `id`.

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/migrations/
git commit -m "feat(bot): migration — create model_configs"
```

---

## Task 2: `characters`

**Files:**
- Create: `bot/db/migrations/<ts>_create_characters.sql`

- [ ] **Step 1: Generate**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-new name=create_characters
```

- [ ] **Step 2: Contents**

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE characters (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         TEXT        NOT NULL UNIQUE,
    name         TEXT        NOT NULL UNIQUE,
    blurb        TEXT        NOT NULL,
    avatar_url   TEXT        NULL,
    base_prompt  TEXT        NOT NULL,
    bot_username TEXT        NOT NULL,
    is_active    BOOLEAN     NOT NULL DEFAULT FALSE,
    position     INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE characters;
-- +goose StatementEnd
```

- [ ] **Step 3: Apply + verify**

```bash
make migrate-up
docker compose exec -T postgres psql -U rollton rollton -c '\d characters'
```
Expected: table created with unique constraints on `slug` and `name`.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/migrations/
git commit -m "feat(bot): migration — create characters"
```

---

## Task 3: `contexts`

**Files:**
- Create: `bot/db/migrations/<ts>_create_contexts.sql`

- [ ] **Step 1: Generate**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-new name=create_contexts
```

- [ ] **Step 2: Contents**

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE contexts (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id       UUID        NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    model_config_id    UUID        NOT NULL REFERENCES model_configs(id) ON DELETE RESTRICT,
    slug               TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    description        TEXT        NOT NULL,
    prompt             TEXT        NOT NULL,
    is_active          BOOLEAN     NOT NULL DEFAULT FALSE,
    is_age_restricted  BOOLEAN     NOT NULL DEFAULT FALSE,
    position           INTEGER     NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (character_id, slug),
    UNIQUE (character_id, name)
);

CREATE INDEX contexts_character_id_idx ON contexts (character_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE contexts;
-- +goose StatementEnd
```

- [ ] **Step 3: Apply + verify**

```bash
make migrate-up
docker compose exec -T postgres psql -U rollton rollton -c '\d contexts'
```
Expected: table created with both unique constraints `(character_id, slug)` and `(character_id, name)`, plus index on `character_id`, plus the two FKs.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/migrations/
git commit -m "feat(bot): migration — create contexts (FK characters + model_configs)"
```

---

## Task 4: `chats`

**Files:**
- Create: `bot/db/migrations/<ts>_create_chats.sql`

- [ ] **Step 1: Generate**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-new name=create_chats
```

- [ ] **Step 2: Contents**

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE chats (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    character_id  UUID        NOT NULL REFERENCES characters(id) ON DELETE RESTRICT,
    context_id    UUID        NOT NULL REFERENCES contexts(id) ON DELETE RESTRICT,
    status        TEXT        NOT NULL DEFAULT 'active',
    summary       TEXT        NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX chats_user_status_updated_idx ON chats (user_id, status, updated_at DESC);
CREATE INDEX chats_character_id_idx       ON chats (character_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE chats;
-- +goose StatementEnd
```

- [ ] **Step 3: Apply + verify**

```bash
make migrate-up
docker compose exec -T postgres psql -U rollton rollton -c '\d chats'
```
Expected: table with 3 FKs (user_id CASCADE, character_id RESTRICT, context_id RESTRICT), both indexes present.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/migrations/
git commit -m "feat(bot): migration — create chats (FK users + characters + contexts)"
```

---

## Task 5: `tg_messages`

**Files:**
- Create: `bot/db/migrations/<ts>_create_tg_messages.sql`

- [ ] **Step 1: Generate**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-new name=create_tg_messages
```

- [ ] **Step 2: Contents**

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE tg_messages (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id              UUID        NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role                 TEXT        NOT NULL,
    content              TEXT        NOT NULL,
    telegram_message_id  BIGINT      NULL,
    attachment_kind      TEXT        NULL,
    attachment_file_id   TEXT        NULL,
    llm_model            TEXT        NULL,
    llm_tokens_in        INTEGER     NULL,
    llm_tokens_out       INTEGER     NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tg_messages_chat_created_idx ON tg_messages (chat_id, created_at);
CREATE UNIQUE INDEX tg_messages_chat_tgmsg_unique
    ON tg_messages (chat_id, telegram_message_id)
    WHERE telegram_message_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE tg_messages;
-- +goose StatementEnd
```

- [ ] **Step 3: Apply + verify**

```bash
make migrate-up
docker compose exec -T postgres psql -U rollton rollton -c '\d tg_messages'
```
Expected: table with chronological index + the partial unique index on `(chat_id, telegram_message_id) WHERE telegram_message_id IS NOT NULL`.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/migrations/
git commit -m "feat(bot): migration — create tg_messages (FK chats)"
```

---

## Task 6: `extend_users` (active_chat_id + age gate)

**Files:**
- Create: `bot/db/migrations/<ts>_extend_users.sql`

- [ ] **Step 1: Generate**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-new name=extend_users
```

- [ ] **Step 2: Contents**

```sql
-- +goose Up
-- +goose StatementBegin
ALTER TABLE users
    ADD COLUMN active_chat_id   UUID        NULL REFERENCES chats(id) ON DELETE SET NULL,
    ADD COLUMN is_adult         BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN age_verified_at  TIMESTAMPTZ NULL;

CREATE INDEX users_active_chat_id_idx ON users (active_chat_id) WHERE active_chat_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS users_active_chat_id_idx;
ALTER TABLE users
    DROP COLUMN IF EXISTS active_chat_id,
    DROP COLUMN IF EXISTS is_adult,
    DROP COLUMN IF EXISTS age_verified_at;
-- +goose StatementEnd
```

- [ ] **Step 3: Apply + verify**

```bash
make migrate-up
docker compose exec -T postgres psql -U rollton rollton -c '\d users'
```
Expected: `users` now shows the three new columns (`active_chat_id`, `is_adult`, `age_verified_at`) and the new partial index.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/migrations/
git commit -m "feat(bot): migration — extend users with active_chat_id + age gate"
```

---

## Task 7: `seed_default_model_config`

**Files:**
- Create: `bot/db/migrations/<ts>_seed_default_model_config.sql`

- [ ] **Step 1: Generate**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
set -a; source .env; set +a
make migrate-new name=seed_default_model_config
```

- [ ] **Step 2: Contents**

```sql
-- +goose Up
-- +goose StatementBegin
INSERT INTO model_configs (slug, display_name, provider, model, temperature, max_tokens, is_active)
VALUES (
    'default-fast',
    'Default (Claude Haiku 4.5)',
    'openrouter',
    'anthropic/claude-haiku-4.5',
    0.8,
    2048,
    TRUE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DELETE FROM model_configs WHERE slug = 'default-fast';
-- +goose StatementEnd
```

- [ ] **Step 3: Apply + verify**

```bash
make migrate-up
docker compose exec -T postgres psql -U rollton rollton -c \
  "SELECT slug, display_name, provider, model FROM model_configs;"
```
Expected: one row — `default-fast | Default (Claude Haiku 4.5) | openrouter | anthropic/claude-haiku-4.5`.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/migrations/
git commit -m "feat(bot): migration — seed default-fast model_config"
```

---

## Task 8: Regenerate schema snapshot + final verification

**Files:**
- Modify: `bot/db/schema/schema.sql`

- [ ] **Step 1: Regenerate `schema.sql` via the docker-based dump script**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
make schema-dump
```
Expected: `✓ db/schema/schema.sql regenerated`. The file now contains every table created by the migrations (users + the 5 new ones), with all constraints and indexes.

- [ ] **Step 2: Quick visual check of `schema.sql`**

```bash
grep -E '^CREATE TABLE|^CREATE INDEX|^CREATE UNIQUE' db/schema/schema.sql | sort
```
Expected: lists `CREATE TABLE` for `users`, `model_configs`, `characters`, `contexts`, `chats`, `tg_messages`, plus the indexes (`chats_*_idx`, `contexts_character_id_idx`, `tg_messages_*`, `users_active_chat_id_idx`, etc.).

- [ ] **Step 3: Confirm sqlc still runs (no-op, but should not error)**

```bash
sqlc generate
```
Expected: succeeds, no new generated files (because no new queries reference these tables yet).

- [ ] **Step 4: Confirm Go build still passes**

```bash
go build ./...
```
Expected: exits 0.

- [ ] **Step 5: Run full test suite**

```bash
make test
```
Expected: all PASS (none of the new tables are referenced by Go code yet, so nothing regressed).

- [ ] **Step 6: Final migration status**

```bash
make migrate-status
```
Expected: 8 migrations Applied — the original `create_users` + the 7 from this plan.

- [ ] **Step 7: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/db/schema/schema.sql
git commit -m "chore(bot): regenerate schema.sql snapshot from character-chats migrations"
```

- [ ] **Step 8: Push when ready**

```bash
git push origin main
```

---

## Out-of-scope (intentional, deferred)

Per the spec, this plan deliberately does NOT include:

- **Domain types** for the new entities (`Character`, `Context`, `Chat`, `TgMessage`, `ModelConfig`). They land alongside the first service that reads them.
- **sqlc queries** against the new tables. Same reason — added per-feature.
- **API endpoints** (`POST /api/v1/chats`, `GET /api/v1/characters`, etc.). Separate spec / plan.
- **Mini-app changes** (character picker UI, context picker, age gate modal). Separate spec.
- **`/api/v1/me/verify-age` endpoint.** Separate spec.
- **Seed characters / contexts.** The system has zero characters after this plan completes; populate via SQL inserts when first feature lands.
- **Backfill of any user data.** All existing users get `is_adult=false` and `active_chat_id=NULL` by virtue of the column defaults.

## Rollback story (if something breaks)

If any migration's Up succeeds but later turns out wrong, roll back to a known-good state:

```bash
# Roll back N migrations (e.g. all 7 from this plan):
goose -dir db/migrations postgres "$DATABASE_URL" down-to <last-known-good-version>
```

(Replace `<last-known-good-version>` with the timestamp of the migration you want to stop AFTER applying.)

Per-migration Down statements are written; rollback is data-destructive but functionally correct.

## Open items resolved at execution time

- The exact `goose create` timestamp prefixes — generated at run time.
- Whether `anthropic/claude-haiku-4.5` is the current best-fit model for the default seed. If a newer Haiku has shipped by execution time, bump the model string in Task 7's SQL.
- Whether `0.8` is the right default temperature. Reasonable starting point; tune in the `model_configs` row later without a migration.
