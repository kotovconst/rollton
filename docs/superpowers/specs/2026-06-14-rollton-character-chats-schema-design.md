# Character chats — data schema design

**Date:** 2026-06-14
**Status:** Approved (design phase)
**Scope:** Schema for the Character.AI-in-Telegram product. Covers characters, contexts, model configs, chats, conversation history, age-gate, and the active-chat pointer needed for the launcher-only operating mode.

## 1. Context

The product is shaped: users open `rolltonchatbot`, pick a character and a context via the mini-app, then talk to that character. The schema here makes that workable. It deliberately models the **eventual** per-character-bot architecture (option B from the brainstorm) but operates today in **single-launcher-bot mode** (option C) — schema doesn't change between modes; only one column on `users` (`active_chat_id`) is C-specific.

## 2. Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Operating mode | C (launcher is the only Telegram bot) | Pre-revenue; defers per-character-bot ops + Telegram bot setup |
| Schema target | B-ready (per-character bots) | Migration to B is a data update (set `characters.bot_username` per row) + drop one column, not a schema move |
| Context lifetime in a chat | Frozen — switching context starts a new chat | Simpler reasoning, simpler memory window, cleaner session UX |
| Model config | Per-context, mandatory FK | Each context picks its own model + sampling params (different vibes need different models) |
| LLM provider | OpenRouter via a `provider` enum; schema is provider-agnostic | One client class today; multi-provider later if needed |
| Age gate | Self-attested `is_adult` boolean on users; `is_age_restricted` boolean on contexts; backend enforces at chat creation | Industry-standard for Telegram adult content; no DOB storage |
| Conversation memory | Schema-level: nullable `chats.summary`; code-level: window last N + summarize older | Schema scales without forcing immediate summarization logic |
| Soft-delete | None for chats/messages (keep history forever); `is_active` flag on characters / contexts / model_configs for staging | Pre-revenue, retention > deletion |
| Slug strategy | All admin-facing entities have a `slug`; deep-links use slugs, not UUIDs | Stable across renames; URL-safe |
| Chat creation moment | Mini-app `POST /api/v1/chats` creates it before any messaging happens | No dangling chats on user abandon; same flow works for C and B |
| Multi-tenancy | None | Single global catalog of characters and contexts |
| Per-character bot tokens | Live in SSM/env, **not in DB** | `characters.bot_username` references the bot by handle; tokens stay out of Postgres |

## 3. Schema

### 3.1 `model_configs` (new)

```sql
CREATE TABLE model_configs (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          TEXT        NOT NULL UNIQUE,
    display_name  TEXT        NOT NULL,
    provider      TEXT        NOT NULL,           -- 'openrouter' for now
    model         TEXT        NOT NULL,           -- e.g. 'anthropic/claude-haiku-4.5'
    temperature   DOUBLE PRECISION NULL,
    top_p         DOUBLE PRECISION NULL,
    max_tokens    INTEGER     NULL,
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Named, reusable model recipes. One context = exactly one model_config. Seeded with at least one row (`default-fast`).

### 3.2 `characters` (new)

```sql
CREATE TABLE characters (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         TEXT        NOT NULL UNIQUE,
    name         TEXT        NOT NULL UNIQUE,
    blurb        TEXT        NOT NULL,
    avatar_url   TEXT        NULL,
    base_prompt  TEXT        NOT NULL,
    bot_username TEXT        NOT NULL,   -- C: all 'rolltonchatbot'; B: per-character handle
    is_active    BOOLEAN     NOT NULL DEFAULT FALSE,
    position     INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### 3.3 `contexts` (new)

```sql
CREATE TABLE contexts (
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id       UUID        NOT NULL REFERENCES characters(id) ON DELETE CASCADE,
    model_config_id    UUID        NOT NULL REFERENCES model_configs(id) ON DELETE RESTRICT,
    slug               TEXT        NOT NULL,
    name               TEXT        NOT NULL,
    description        TEXT        NOT NULL,
    prompt             TEXT        NOT NULL,                -- appended to character.base_prompt
    is_active          BOOLEAN     NOT NULL DEFAULT FALSE,
    is_age_restricted  BOOLEAN     NOT NULL DEFAULT FALSE,
    position           INTEGER     NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (character_id, slug),
    UNIQUE (character_id, name)
);

CREATE INDEX contexts_character_id_idx ON contexts (character_id);
```

`ON DELETE RESTRICT` on `model_config_id` — deleting a model_config that contexts reference must be explicit.

### 3.4 `chats` (new)

```sql
CREATE TABLE chats (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    character_id  UUID        NOT NULL REFERENCES characters(id) ON DELETE RESTRICT,
    context_id    UUID        NOT NULL REFERENCES contexts(id) ON DELETE RESTRICT,
    status        TEXT        NOT NULL DEFAULT 'active',  -- 'active' | 'archived'
    summary       TEXT        NULL,                        -- compressed memory; populated later
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX chats_user_status_updated_idx ON chats (user_id, status, updated_at DESC);
CREATE INDEX chats_character_id_idx       ON chats (character_id);
```

Character / context are RESTRICT — you can't drop a character/context that has chats. You disable them (`is_active=false`) instead.

### 3.5 `tg_messages` (new)

```sql
CREATE TABLE tg_messages (
    id                   UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id              UUID        NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    role                 TEXT        NOT NULL,   -- 'user' | 'assistant' | 'system'
    content              TEXT        NOT NULL,
    telegram_message_id  BIGINT      NULL,
    attachment_kind      TEXT        NULL,       -- 'photo' | 'voice' | 'document' | NULL
    attachment_file_id   TEXT        NULL,       -- Telegram file_id pointer
    llm_model            TEXT        NULL,       -- snapshot of model used (assistant only)
    llm_tokens_in        INTEGER     NULL,
    llm_tokens_out       INTEGER     NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX tg_messages_chat_created_idx ON tg_messages (chat_id, created_at);
CREATE UNIQUE INDEX tg_messages_chat_tgmsg_unique
    ON tg_messages (chat_id, telegram_message_id)
    WHERE telegram_message_id IS NOT NULL;
```

Partial unique index protects against duplicate ingestion when Telegram retries delivery of the same update. Outbound LLM messages (where `telegram_message_id` is NULL until we Send them, or even after if we don't capture the ID) are exempt.

### 3.6 `users` (modified — three new columns)

```sql
ALTER TABLE users
    ADD COLUMN active_chat_id   UUID        NULL REFERENCES chats(id) ON DELETE SET NULL,
    ADD COLUMN is_adult         BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN age_verified_at  TIMESTAMPTZ NULL;

CREATE INDEX users_active_chat_id_idx ON users (active_chat_id) WHERE active_chat_id IS NOT NULL;
```

- **`active_chat_id`** — C-only. Points at the chat that rolltonchatbot's update handler should service for this user. When a user switches characters in the mini-app, this updates atomically with the new chat. **Drop this column when migrating to B.**
- **`is_adult`** — self-attested 18+ flag.
- **`age_verified_at`** — when the attestation was made (audit trail).

`ON DELETE SET NULL` on `active_chat_id` because if a chat is ever deleted, the user shouldn't be orphan-pointing into the void.

## 4. Migration order (goose timestamps)

Each is its own goose file. Filenames use placeholder timestamps; goose generates real ones.

1. `<ts>_create_model_configs.sql`
2. `<ts>_create_characters.sql`
3. `<ts>_create_contexts.sql` (FK to characters + model_configs)
4. `<ts>_create_chats.sql` (FK to users + characters + contexts)
5. `<ts>_create_tg_messages.sql` (FK to chats)
6. `<ts>_extend_users.sql` (active_chat_id + is_adult + age_verified_at, in one migration)
7. `<ts>_seed_default_model_config.sql` (insert `default-fast` row pointing at a Claude Haiku 4.5 model id)

The FK dependencies dictate the order — chats can't reference characters until characters exists; tg_messages can't reference chats until chats exists.

## 5. Data flow

### 5.1 Under C (today)

```
User → rolltonchatbot → /start
         │
         ▼
mini-app loads (TMA auth wires up — separate spec)
         │
         ▼
GET /api/v1/characters                          → list active characters
         │
         ▼
User picks a character
         │
         ▼
GET /api/v1/characters/:slug/contexts           → list active contexts for that character
                                                  (filtered/tagged by user.is_adult)
         │
         ▼
User picks a context
         │
   if context.is_age_restricted && !user.is_adult:
      mini-app shows age gate; POST /api/v1/me/verify-age → user.is_adult = true
         │
         ▼
POST /api/v1/chats { character_id, context_id }
   backend (rolltonchatbot's API):
     - INSERT chats (user_id, character_id, context_id, status='active')
     - UPDATE users SET active_chat_id = new_chat.id WHERE id = user.id
     - returns the chat
         │
         ▼
Mini-app WebApp.close()
         │
         ▼
User types in rolltonchatbot Telegram chat
         │
         ▼
Telegram middleware:
  - ensure user registered (existing)
  - load user.active_chat_id → chat
  - load chat.character, chat.context, chat.context.model_config
  - load last N tg_messages for chat
  - build LLM prompt: character.base_prompt + context.prompt + last N + new user msg
  - call OpenRouter
  - INSERT tg_messages role='user' (with telegram_message_id)
  - INSERT tg_messages role='assistant' (with llm_model, tokens snapshots)
  - Send reply via Telegram
```

### 5.2 Under B (the migration target)

Steps 1-4 above are identical. Step 5 changes to:

```
Mini-app calls openCharacterBot(character.bot_username, "chat_" + chat.id)
   → Telegram opens t.me/<character_bot>?start=chat_<id>
   → Per-character bot's /start handler:
       - parse start param → chat_id
       - load chat, verify ownership (chat.user_id == sender's user_id)
       - continue thread in this DM
   → users.active_chat_id is no longer consulted
```

### 5.3 The migration itself (data-only)

When a character is ready to get its own dedicated Telegram bot:

1. Create the bot via @BotFather, get token.
2. Store token in SSM: `/rollton/prod/<character_slug>_bot/telegram_token`.
3. Update DB: `UPDATE characters SET bot_username = '<new_handle>' WHERE slug = '<character_slug>';`
4. Deploy a new instance of the `character-bot` binary (parameterized by `CHARACTER_ID` env).
5. Update mini-app's "Open chat" button to deep-link when `bot_username != 'rolltonchatbot'`.
6. Eventually, when all characters are on their own bots: drop `users.active_chat_id`.

No table is moved. No data is rewritten. No FK changes. The schema is the same.

## 6. Indexes summary

| Table | Index | Type | Purpose |
|---|---|---|---|
| `users.telegram_id` | unique | btree | (existing) auth lookup |
| `users.active_chat_id` | partial btree | btree WHERE NOT NULL | join from users to chats |
| `characters.slug` | unique | btree | URL/API lookup |
| `characters.name` | unique | btree | dedupe display |
| `contexts.character_id` | btree | btree | list contexts per character |
| `contexts.(character_id, slug)` | unique | btree | URL lookup, per-character stable id |
| `contexts.(character_id, name)` | unique | btree | dedupe display |
| `model_configs.slug` | unique | btree | admin reference |
| `chats.(user_id, status, updated_at DESC)` | btree | btree | "list my chats" |
| `chats.character_id` | btree | btree | analytics |
| `tg_messages.(chat_id, created_at)` | btree | btree | chronological replay |
| `tg_messages.(chat_id, telegram_message_id)` | unique | partial btree WHERE NOT NULL | webhook dedupe |

## 7. Out-of-scope (explicit non-goals)

- **Subscription / billing tables.** Will be a separate spec — `subscriptions`, `entitlements`, integration with a payment provider.
- **Admin UI / CMS.** No UI for character/context CRUD in this spec. Initial data goes in via SQL seed files; future admin tooling is a separate spec.
- **Conversation summarization logic.** `chats.summary` column is reserved; the algorithm (when to summarize, what model, what trigger) is a separate code-level feature.
- **Message editing / deletion via Telegram.** User edits in Telegram don't propagate to `tg_messages` in this spec.
- **Attachments storage.** Schema records `attachment_kind` + `attachment_file_id` (Telegram pointer). Downloading and re-hosting is a separate spec.
- **Multi-language prompts.** `users.language_code` exists but contexts/characters don't have language variants yet.
- **Soft-delete / GDPR deletion.** Hard-delete on `ON DELETE CASCADE` from users → chats → tg_messages. No retention policy yet.
- **Per-user-per-character favorites / notes.** Could be a future `user_character_state` table; not now.
- **Chat title / auto-naming.** Chats are identified by `character.name + " — " + context.name` in lists. No explicit title.
- **Rate limiting on chat creation.** Code-level; no schema impact.
- **OpenRouter API key in DB.** Stays in env / SSM.

## 8. Open items resolved at execution time

- Initial `model_configs` seed row's `model` value — pick the current best Claude Haiku ID (`anthropic/claude-haiku-4.5` at time of writing; bump if a newer/faster option exists when the migration runs).
- `position` semantics — application sorts by `position ASC, created_at ASC`. App enforces consistency; DB allows ties.
- `tg_messages.attachment_kind` enum values are application-enforced. Adding a CHECK constraint or a Postgres ENUM type is deferred — keep it `TEXT` for evolvability.
- Whether to add a CHECK constraint on `tg_messages.role` (`IN ('user', 'assistant', 'system')`) — defer; application enforces, easier to add new roles (e.g., `tool`) without migration.
- Migration timestamp ordering — goose generates timestamps at run time; the listed order is the order to run `make migrate-new`.
- Whether to add an `is_default` flag to `model_configs` so contexts can fall back to it — for now every context has a mandatory FK; revisit if seeding bulk contexts becomes painful.
