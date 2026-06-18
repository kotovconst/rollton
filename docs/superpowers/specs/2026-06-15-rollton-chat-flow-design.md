# Rollton Chat-Flow Service — Design

## 1. Goal

Land the LLM-backed chat loop for **character bots**: when a user texts a character bot (e.g. `@snoopdoggbot`), the bot replies with an LLM-generated assistant turn that respects the active character's prompt, the selected context's prompt, and recent conversation history.

This slice ships the service + two running character bots (Snoop + Sherlock) plus the binary that hosts them. It does **not** ship context-switching UX, RAG, or any per-character override beyond the character row itself.

## 2. Scope

**In:**

- A new shared `services.ChatFlowService.Handle(...)` that does the whole user-msg → LLM → assistant-msg round-trip.
- A new binary `cmd/characterbots` that loads active characters from the DB on boot, reads each one's bot token from env (`BOT_TOKEN_<UPPER(SLUG)>` with `-` → `_` normalization), and spawns one Telegram long-poll goroutine per character via a `runCharactersBot` function.
- Persistence of every inbound user turn and every outbound assistant turn into `tg_messages`.
- Per-chat ordering relies on the existing `tgbot.Bot.Run` loop being single-threaded per character bot (one update handled to completion before the next is read). No application-level mutex is added.
- Idempotency against Telegram redelivery via the existing partial unique index on `(chat_id, telegram_message_id)`.
- Operator-facing structured logs distinguishing the four OpenRouter sentinel classes.
- User-facing generic error reply when the LLM fails.
- A `tgbot.WithTyping` wrap around the LLM call so the user sees "typing…" continuously while waiting.

**Out (deferred to future slices):**

- Context-switching UX (slash commands, inline keyboards, TMA wiring inside character bots).
- RAG over chat history. The `tg_messages.embedding` column is NOT added in this slice — adding it now is YAGNI without pgvector + embedding pipeline.
- Per-character override of the history window size.
- Non-text input handling (stickers, photos, voice, etc.) beyond silent ignore.
- Cost tracking / spend caps per user.
- Streaming responses (locked out by the OpenRouter slice's single-response decision).
- Multi-instance / multi-process deployments. Correctness assumes a single `cmd/characterbots` process; concurrent writes from a second process are not handled.
- Application-level per-chat locking. Ordering is delegated to the existing single-threaded `tgbot.Bot.Run` loop. Any future caller path (TMA write-into-chat, webhook fan-out) should add idempotency-key + cache dedup, not re-introduce an in-memory mutex map.
- Edit/delete propagation for either side.

## 3. Architecture overview

```
┌─────────────────────────────────────────────────────────────────┐
│  cmd/characterbots                                              │
│  └─ main.go  ─→  builds Deps, calls runCharactersBot            │
│                                                                 │
│  internal/bots/characterbots                                    │
│  ├─ characterbots.go (runCharactersBot)                         │
│  │   ├─ queries.ListActiveCharacters(ctx)                       │
│  │   ├─ for each: token := os.Getenv("BOT_TOKEN_" + UPPER(slug))│
│  │   │            if "" → return error (fail fast)              │
│  │   │            start tgbot.Bot in goroutine                  │
│  │   │            register textHandler{characterID, svc}        │
│  │   └─ errgroup.Wait()                                         │
│  │                                                              │
│  └─ handlers/telegram/text_handler.go                           │
│      Handle(ctx) → builds reply(...) closure that calls         │
│                    tgbot.WithTyping + c.api.Send,               │
│                    delegates to ChatFlowService.Handle          │
│                                                                 │
│  internal/core/services/chat_flow_service.go                    │
│  └─ ChatFlowService.Handle(ctx, user, characterID, text,        │
│                            tgMessageID, reply) error            │
│      ├─ findOrCreateChat                                        │
│      ├─ INSERT user msg (ON CONFLICT DO NOTHING)                │
│      │   └─ if conflict + assistant reply exists → return       │
│      │      else continue (crash-before-reply path)             │
│      ├─ build prompt from base_prompt + context.prompt + hist   │
│      ├─ openrouter.Complete                                     │
│      │   ├─ on sentinel error → reply(genericErr), log, return  │
│      │   └─ on success → reply(text), INSERT assistant msg,     │
│      │                   UPDATE chats.updated_at = NOW()        │
│      └─ done                                                    │
└─────────────────────────────────────────────────────────────────┘
```

Everything in `internal/bots/characterbots/` is character-agnostic. The character identity is determined entirely by which goroutine received the update; the handler holds `characterID uuid.UUID` from construction.

## 4. File layout

```
bot/
├── cmd/characterbots/
│   └── main.go                                       # NEW
├── internal/bots/characterbots/
│   ├── characterbots.go                              # NEW: runCharactersBot(ctx, deps) error
│   └── handlers/telegram/
│       ├── text_handler.go                           # NEW
│       └── test/text_handler_test.go                 # NEW
├── internal/core/services/
│   ├── chat_flow_service.go                          # NEW
│   └── test/chat_flow_service_test.go                # NEW (pgxmock + fake openrouter)
├── internal/core/ports/
│   ├── chat_flow_service.go                          # NEW: ChatFlowService interface
│   └── openrouter_client.go                          # NEW: OpenRouterClient interface
├── db/queries/
│   └── chat_flow.sql                                 # NEW: 10 queries (see §6)
└── .env.example                                       # MODIFIED: document BOT_TOKEN_*, LLM_HISTORY_WINDOW
```

## 5. `ChatFlowService.Handle` — signature and flow

```go
package services

type ChatFlowService struct {
    q          ports.Queries
    or         ports.OpenRouterClient
    log        *slog.Logger
    historyN   int32
}

// reply is provided by the handler. It sends the given text via Telegram
// (wrapped in WithTyping for the LLM-call window) and returns the Telegram
// message id of the first sent chunk (used for the assistant-msg insert).
type ReplyFunc func(text string) (tgMessageID int64, err error)

func (s *ChatFlowService) Handle(
    ctx context.Context,
    user domain.User,
    characterID uuid.UUID,
    text string,
    tgUserMessageID int64,
    reply ReplyFunc,
) error
```

The handler chooses the `reply` closure so the service stays free of `tgbotapi`. The handler also wraps the whole `Handle` call in `tgbot.WithTyping(c, func() error { return svc.Handle(...) })`.

### Step-by-step

1. **Resolve chat + character + context + modelConfig.**
   - `q.GetMostRecentChatForUserCharacter(ctx, user.ID, characterID)` → returns chat with joined context + character + model_config.
   - If no row: `q.GetDefaultContextForCharacter(ctx, characterID)` (lowest `position`, `is_active=true`), then `q.InsertChat(ctx, user.ID, ctx.ID)` returning the new chat. Re-resolve joined view.

2. **INSERT user message with idempotency.**
   ```sql
   INSERT INTO tg_messages (chat_id, role, content, telegram_message_id)
   VALUES ($1, 'user', $2, $3)
   ON CONFLICT (chat_id, telegram_message_id)
       WHERE telegram_message_id IS NOT NULL
   DO NOTHING
   RETURNING id, created_at;
   ```
   - **Row returned** → fresh user turn. Continue.
   - **No row returned** (duplicate redelivery):
     - `q.AssistantReplyExistsAfter(ctx, chat.ID, originalUserMsgCreatedAt)` — needs a follow-up SELECT to get the original user msg's `created_at` first.
     - If `true` → already replied; `return nil`.
     - If `false` → crash-before-reply; fetch the existing user msg, continue with the LLM call as normal.

3. **Build the prompt** (§7).

4. **Call OpenRouter.**
   ```go
   resp, err := s.or.Complete(ctx, openrouter.ChatRequest{
       Model:       modelConfig.Model,
       Temperature: modelConfig.Temperature,
       TopP:        modelConfig.TopP,
       MaxTokens:   modelConfig.MaxTokens,
       Messages:    messages,
   })
   ```

5. **On LLM error** (any of `ErrInvalidAuth`, `ErrInsufficientCredits`, `ErrRateLimited`, `ErrUpstream`):
   - `s.log.Error("chat_flow.failed", "chat_id", chat.ID, "character_slug", char.Slug, "err_class", classify(err), "api_err", apiErrFields(err))`
   - `_, _ = reply(GenericErrorReply)` — best-effort; swallow send error.
   - `return nil` (caller treats this as handled gracefully).

6. **On LLM success:**
   - `firstChunkID, err := reply(resp.Reply)` — handler does the chunking + send.
   - If `reply` returns error: log, return nil (user already sent their msg; we just have an orphan turn). Don't insert assistant row.
   - `q.InsertAssistantMessage(ctx, chat.ID, resp.Reply, firstChunkID, resp.Model, resp.TokensIn, resp.TokensOut)`
   - `q.TouchChatUpdatedAt(ctx, chat.ID)` — bumps `updated_at` so the most-recent-chat lookup keeps working.
   - `s.log.Info("chat_flow.complete", "chat_id", chat.ID, "character_slug", char.Slug, "model", resp.Model, "tokens_in", resp.TokensIn, "tokens_out", resp.TokensOut)`
   - `return nil`.

### Ordering rationale

- **No DB transaction wraps user-msg + LLM + assistant-msg.** They are three independent writes; an open tx held across an outbound HTTP call to OpenRouter would block the connection for the full LLM duration and complicate `WithTyping`. If the assistant insert fails after a successful TG send, an orphan turn appears in logs but the user already saw the reply; that's the right failure mode.
- **TG send happens BEFORE assistant DB insert.** This way the assistant row's `telegram_message_id` is known at insert time. If TG send fails, no assistant row is created.
- **`chats.updated_at` bump is the last step**, after the assistant insert. The bump is what makes the "most-recent chat" rule for "current context" continue to work across messages.

## 6. SQL queries

`db/queries/chat_flow.sql`:

```sql
-- name: GetMostRecentChatForUserCharacter :one
-- Returns chat + joined context + character + model_config for the most-recent
-- chat in any context belonging to the given character.
SELECT
    c.id AS chat_id, c.status AS chat_status, c.summary AS chat_summary,
    c.updated_at AS chat_updated_at,
    ctx.id AS context_id, ctx.slug AS context_slug, ctx.name AS context_name,
    ctx.prompt AS context_prompt, ctx.is_age_restricted,
    ch.id AS character_id, ch.slug AS character_slug, ch.name AS character_name,
    ch.base_prompt AS character_base_prompt,
    mc.id AS model_config_id, mc.slug AS model_slug, mc.model AS model_name,
    mc.temperature, mc.top_p, mc.max_tokens
FROM chats c
JOIN contexts ctx ON ctx.id = c.context_id
JOIN characters ch ON ch.id = ctx.character_id
JOIN model_configs mc ON mc.id = ctx.model_config_id
WHERE c.user_id = $1 AND ch.id = $2
ORDER BY c.updated_at DESC
LIMIT 1;

-- name: GetDefaultContextForCharacter :one
-- Lowest-position active context for the character.
SELECT id, slug, name, prompt, is_age_restricted, model_config_id
FROM contexts
WHERE character_id = $1 AND is_active = true
ORDER BY position ASC, created_at ASC
LIMIT 1;

-- name: InsertChat :one
INSERT INTO chats (user_id, context_id)
VALUES ($1, $2)
RETURNING id, status, summary, updated_at;

-- name: InsertUserMessageIdempotent :one
INSERT INTO tg_messages (chat_id, role, content, telegram_message_id)
VALUES ($1, 'user', $2, $3)
ON CONFLICT (chat_id, telegram_message_id)
    WHERE telegram_message_id IS NOT NULL
DO NOTHING
RETURNING id, created_at;

-- name: GetUserMessageByTelegramID :one
SELECT id, created_at
FROM tg_messages
WHERE chat_id = $1 AND telegram_message_id = $2 AND role = 'user';

-- name: AssistantReplyExistsAfter :one
SELECT EXISTS(
    SELECT 1 FROM tg_messages
    WHERE chat_id = $1 AND role = 'assistant' AND created_at > $2
) AS exists;

-- name: ListRecentMessages :many
SELECT role, content, created_at
FROM tg_messages
WHERE chat_id = $1
  AND role IN ('user', 'assistant')
ORDER BY created_at DESC
LIMIT $2;

-- name: InsertAssistantMessage :exec
INSERT INTO tg_messages
    (chat_id, role, content, telegram_message_id, llm_model, llm_tokens_in, llm_tokens_out)
VALUES ($1, 'assistant', $2, $3, $4, $5, $6);

-- name: TouchChatUpdatedAt :exec
UPDATE chats SET updated_at = NOW() WHERE id = $1;

-- name: ListActiveCharactersWithSlug :many
-- Used by runCharactersBot at boot.
SELECT id, slug, name, bot_username
FROM characters
WHERE is_active = true
ORDER BY position ASC;
```

No new indexes are needed. The existing `chats_user_status_updated_idx`, `tg_messages_chat_created_idx`, `tg_messages_chat_tgmsg_unique`, `contexts_character_id_idx` all cover the workload.

## 7. Prompt assembly

```
Messages[0]   = { role: system,    content: character.base_prompt + "\n\n" + context.prompt }
Messages[1..] = chronological history from ListRecentMessages($chat_id, $LLM_HISTORY_WINDOW), reversed in Go
```

Rules:

- `LLM_HISTORY_WINDOW` is an env var; default `20`; global (no per-character override in v1).
- `ListRecentMessages` filters `role IN ('user', 'assistant')` so the system message is never duplicated from history.
- Map `tg_messages.role` to `openrouter.Role`: `'user' → RoleUser`, `'assistant' → RoleAssistant`. Any other value → skip (defensive).
- The just-inserted user turn is the final entry of `Messages` (it's part of the last-N).
- No proactive token-budget truncation. If OpenRouter ever returns `ErrUpstream` for "context too long," that's a follow-up.

## 8. Per-chat serialization — relied on the bot loop, not app code

`tgbot.Bot.Run` processes updates sequentially per character bot (`<-updates` → `handler(c)` synchronously, then the loop reads the next update). So within one character's goroutine, two `Handle` calls for the same chat cannot overlap.

Cross-character concurrency exists (different bots run in different goroutines), but no two character bots share a chat row (each chat is bound to one character's context), so cross-character concurrency never targets the same chat.

This means no application-level mutex is required today. The idempotency-on-Telegram-redelivery case is handled at the DB layer by the partial unique index on `(chat_id, telegram_message_id)`.

**If we ever introduce additional caller paths** (TMA HTTP endpoint into the chat flow, webhook mode, fan-out goroutines in the dispatcher), we should add idempotency-key + cache-based dedup at that boundary rather than re-introducing an in-memory mutex map. That's a separate slice when the need arises.

**Explicit non-goal:** cross-process locking. The deployment model is one process; this is documented as a constraint.

## 9. Bootstrap (`runCharactersBot`)

```go
func runCharactersBot(ctx context.Context, deps Deps) error {
    chars, err := deps.Queries.ListActiveCharactersWithSlug(ctx)
    if err != nil { return fmt.Errorf("load characters: %w", err) }

    g, gctx := errgroup.WithContext(ctx)
    for _, ch := range chars {
        ch := ch
        envName := "BOT_TOKEN_" + strings.ToUpper(strings.ReplaceAll(ch.Slug, "-", "_"))
        token := os.Getenv(envName)
        if token == "" {
            return fmt.Errorf("missing env %s for character %q", envName, ch.Slug)
        }
        api, err := tgbotapi.NewBotAPI(token)
        if err != nil { return fmt.Errorf("tg bot for %s: %w", ch.Slug, err) }

        bot := tgbot.NewBot(api)
        bot.OnText(telegram.NewTextHandler(ch.ID, deps.ChatFlowService))
        g.Go(func() error { return bot.Run(gctx) })

        deps.Log.Info("character.started", "slug", ch.Slug, "username", ch.BotUsername)
    }
    return g.Wait()
}
```

Failure modes:

- Missing token for an active character → **refuse to start** (fail-fast; don't silently skip).
- Telegram API auth failure for any token → propagate, kill the process.
- One character's polling goroutine returning an error → `errgroup` cancels the rest and the process exits with that error.

(Specifics of `tgbot.Bot`'s router-registration API may differ slightly; the implementation plan will reconcile with what currently exists in `pkg/tgbot/`. Conceptually: one bot per character, each with the text handler registered.)

## 10. Telegram handler details

`internal/bots/characterbots/handlers/telegram/text_handler.go`:

```go
const GenericErrorReply = "Something went wrong, try again in a moment."

type TextHandler struct {
    characterID uuid.UUID
    svc         ports.ChatFlowService
}

func (h *TextHandler) Handle(c *tgbot.Context) error {
    u, ok := middleware.UserFromContext(c.Ctx())
    if !ok { return nil } // user middleware should always populate
    msg := c.Update.Message
    if msg == nil || msg.Text == "" { return nil }

    reply := func(text string) (int64, error) {
        // chunk at last \n before 4096, then send sequentially; return first chunk MessageID
        return sendChunked(c.API(), msg.Chat.ID, text)
    }
    return tgbot.WithTyping(c, func() error {
        return h.svc.Handle(c.Ctx(), u, h.characterID, msg.Text, int64(msg.MessageID), reply)
    })
}
```

`sendChunked` is a small helper:
- If `len(text) <= 4096`, send once, return that `MessageID`.
- Otherwise split at the last `\n` before 4096 (fall back to a hard cut if no `\n`). Send the first chunk, capture its `MessageID`, then send subsequent chunks (their IDs are not persisted). Return the **first** chunk's `MessageID` for the assistant-msg insert.

The handler ignores non-text updates because `msg == nil || msg.Text == ""` short-circuits. Non-text updates from the router never reach the LLM. (The router-level filter — registering only for text updates — is the cleaner answer; the in-handler guard is belt-and-braces.)

## 11. Error handling — full table

| Trigger                                        | DB side-effect                       | User-facing reply                                        | Log level + class                  |
|------------------------------------------------|--------------------------------------|----------------------------------------------------------|------------------------------------|
| Happy path                                     | user-msg + assistant-msg + touch     | LLM reply (chunked if >4096)                             | `INFO chat_flow.complete`          |
| `ErrInvalidAuth`                               | user-msg only                        | `GenericErrorReply`                                      | `ERROR chat_flow.failed` (class)   |
| `ErrInsufficientCredits`                       | user-msg only                        | `GenericErrorReply`                                      | `ERROR chat_flow.failed` (class)   |
| `ErrRateLimited`                               | user-msg only                        | `GenericErrorReply`                                      | `ERROR chat_flow.failed` (class)   |
| `ErrUpstream`                                  | user-msg only                        | `GenericErrorReply`                                      | `ERROR chat_flow.failed` (class)   |
| `context.Canceled` / `context.DeadlineExceeded`| user-msg only                        | (none — caller shutting down)                            | `WARN chat_flow.cancelled`         |
| Duplicate TG redelivery, assistant exists      | none                                 | none                                                     | `DEBUG chat_flow.duplicate_skip`   |
| Duplicate TG redelivery, no assistant          | user-msg already present; LLM runs   | LLM reply                                                | as success                         |
| TG send fails after LLM success                | user-msg only (no assistant insert)  | (already failed)                                         | `ERROR chat_flow.tg_send_failed`   |
| Non-text input                                 | none                                 | none                                                     | none                               |

The "log + ops response" mapping (paging on `ErrInvalidAuth` / `ErrInsufficientCredits`) is **infrastructure**, not in scope — the log lines have the fields, the pager isn't wired.

## 12. Testing strategy

### `internal/core/services/test/chat_flow_service_test.go` (pgxmock + fake openrouter)

- `Handle_NewChat_AutoCreates_WithDefaultContext`
- `Handle_ExistingChat_Reuses_MostRecent`
- `Handle_HappyPath_PersistsBothMessages_TouchesChatUpdatedAt`
- `Handle_PromptShape_HasSystemFirst_AndChronologicalHistory`
- `Handle_HistoryWindow_TruncatesToN`
- `Handle_DuplicateUserMsg_AssistantReplyExists_Skips`
- `Handle_DuplicateUserMsg_NoReply_RetriesLLM` (crash-before-reply path)
- `Handle_LLM_ErrInvalidAuth_RepliesGenericError_NoAssistantInsert`
- `Handle_LLM_ErrInsufficientCredits_RepliesGenericError_NoAssistantInsert`
- `Handle_LLM_ErrRateLimited_RepliesGenericError_NoAssistantInsert`
- `Handle_LLM_ErrUpstream_RepliesGenericError_NoAssistantInsert`
- `Handle_TGSendFailsAfterLLMSuccess_LeavesOrphanUserMsg_NoAssistantInsert`
- `Handle_PerChatSerialization_TwoConcurrentCallsObserveMutex`

### `internal/bots/characterbots/handlers/telegram/test/text_handler_test.go`

- `TextHandler_TextUpdate_DispatchesToService_WithRightArgs`
- `TextHandler_EmptyOrNilMessage_NoOp`
- `TextHandler_SendChunked_ShortText_SingleSend`
- `TextHandler_SendChunked_LongText_SplitsAtNewline`

### Fakes (new in `internal/core/ports/`)

- `OpenRouterClient` interface: `Complete(ctx, ChatRequest) (ChatResponse, error)`. Production `*openrouter.Client` implements implicitly.
- `Queries` interface: just an alias of the sqlc-generated `Querier` so tests can pass a pgxmock-backed value.

No live OpenRouter tests in this slice; httptest already covers the client end in the prior slice.

## 13. Config additions

`.env.example` additions:

```dotenv
# Character bot tokens — one per active character row.
# Slug "snoop-dogg" → BOT_TOKEN_SNOOP_DOGG (replace `-` with `_`, uppercase).
# Missing token for an active character = startup failure (fail-fast).
BOT_TOKEN_SNOOP_DOGG=
BOT_TOKEN_SHERLOCK_HOLMES=

# LLM history window: number of prior tg_messages rows included in each
# OpenRouter call. Default 20. Global (no per-character override in v1).
LLM_HISTORY_WINDOW=20
```

The slug-to-env normalization (`-` → `_`, upper-case) is documented above and implemented in `runCharactersBot`.

## 14. Out-of-scope (explicit non-goals)

- Context-switching UX in either bot.
- RAG over chat history (`tg_messages.embedding` column NOT added).
- Per-character history-window override.
- Token-budget-based prompt truncation.
- Non-text input handling beyond silent ignore.
- Edit / delete propagation either direction.
- Streaming SSE responses.
- Cross-process / multi-instance per-chat locking.
- Cost tracking, spend caps, usage dashboards.
- Inline keyboards anywhere in this slice.
- Welcome / onboarding messages from character bots (no `/start` handler on character bots in this slice).

## 15. Open items at execution time

- Confirm `pkg/tgbot.Bot` supports the multi-instance use case (one `*tgbotapi.BotAPI` per character, polled concurrently). If the current `tgbot.Bot.Run` has any singleton assumption, fix as part of the implementation plan.
- Confirm sqlc handles the joined `GetMostRecentChatForUserCharacter` cleanly. If the generated row type is awkward (many columns), split into two queries.
- `sendChunked`: confirm Telegram preserves Markdown formatting across chunk boundaries. For v1 we send plain text (no parse mode) so the question is moot, but if a future slice enables MarkdownV2 the chunker needs to be format-aware.
- `runCharactersBot` token-missing behavior: fail-fast is specced. Confirm this aligns with deployment ergonomics (operator must add tokens for every active character before deploy).
