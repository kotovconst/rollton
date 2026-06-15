# OpenRouter client (`pkg/openrouter`) + Telegram typing helper — design

**Date:** 2026-06-15
**Status:** Approved (design phase)
**Scope:** Stand up a minimal OpenRouter HTTP client in `bot/pkg/openrouter` so future chat-flow code can call LLMs. Add a `pkg/tgbot/WithTyping` helper for the "rolltonchatbot is typing..." indicator. **No chat-flow wiring** — that's the next slice.

## 1. Context

The bot has the data schema and `/api/v1/me` + `/api/v1/characters` endpoints. The next user-visible behaviour is "user sends a Telegram message → bot replies in character voice". That requires:

1. An LLM client (this spec).
2. A chat-flow orchestrator (next spec): pick the right character/context from a chat, build the prompt, call the LLM, store the assistant message, send via Telegram.

We split (1) from (2) so the LLM client has no Telegram or service-layer coupling — pure HTTP-over-JSON, testable in isolation.

The `WithTyping` helper is bundled with this slice because it's tiny and ready when the orchestrator lands.

## 2. Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Provider | OpenRouter only | The `model_configs.provider` enum can grow; this package is OpenRouter-specific. |
| Package path | `bot/pkg/openrouter` | Matches the operator-chosen flat layout (no `pkg/llm/` intermediate dir). |
| Response shape | Single response, blocking | Telegram doesn't support streaming UX (rate-limited message edits make it pointless). |
| Request shape | Struct in, struct out | Easier to extend, idiomatic Go, mirrors the rest of the codebase. |
| Optional sampling params | Pointer fields (`*float64`, `*int`) | `nil` cleanly maps to "use OpenRouter default" and to `model_configs.X NULL`. |
| Errors | Typed sentinel + `APIError` carrier | `errors.Is` for class-level branching; `errors.As` to inspect specifics. |
| HTTP client | `*http.Client` injectable via `WithHTTPClient` option | Default `http.DefaultClient` works in production; tests override. |
| Base URL | Overridable via `WithBaseURL` option | Test via `httptest.NewServer` without monkey-patching. |
| App attribution | Send `HTTP-Referer` + `X-Title` headers (OpenRouter convention) | Optional but lets OpenRouter dashboard show per-app usage. |
| Retries / circuit breaking | None in client | Caller decides. `Retry-After` exposed on `APIError` for rate-limit cases. |
| Streaming | Deferred | Not needed for Telegram. |
| Tool / function-calling | Deferred | No use case yet. |
| Multi-modal input | Deferred | No use case yet. |
| Live integration test | Deferred (`//go:build live` tag later) | Needs a real API key + credits. |
| Typing indicator | `pkg/tgbot.WithTyping(c, fn)` — bundled with this slice | Tiny adjacent piece; ~30 lines. |
| New dependencies | None | OpenRouter API is JSON over HTTP; stdlib `net/http` covers it. |

## 3. Public API

### 3.1 `pkg/openrouter/types.go`

```go
package openrouter

type Role string

const (
    RoleSystem    Role = "system"
    RoleUser      Role = "user"
    RoleAssistant Role = "assistant"
)

type Message struct {
    Role    Role   `json:"role"`
    Content string `json:"content"`
}

type ChatRequest struct {
    Model       string    // required, e.g. "anthropic/claude-haiku-4.5"
    Messages    []Message // system message (if any) at Messages[0]
    Temperature *float64  // nil → OpenRouter default
    TopP        *float64  // nil → OpenRouter default
    MaxTokens   *int      // nil → OpenRouter default
}

type ChatResponse struct {
    Model        string // echoed back from OpenRouter; may include a version suffix
    Reply        string // assistant message content (choices[0].message.content)
    TokensIn    int    // usage.prompt_tokens
    TokensOut   int    // usage.completion_tokens
    FinishReason string // "stop" | "length" | "content_filter" | ...
}
```

### 3.2 `pkg/openrouter/client.go`

```go
type Client struct {
    apiKey     string
    httpClient *http.Client
    baseURL    string
    appURL     string
    appName    string
}

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option
func WithBaseURL(u string) Option
func WithApp(url, name string) Option

func New(apiKey string, opts ...Option) *Client

func (c *Client) Complete(ctx context.Context, req ChatRequest) (ChatResponse, error)
```

`Complete` builds the JSON body (omitting nil sampling fields), POSTs to `<baseURL>/chat/completions`, decodes the response, and returns `ChatResponse`. On non-2xx or transport error it returns one of the sentinels joined with an `APIError` (where applicable) — see Section 4.

### 3.3 `pkg/openrouter/errors.go`

```go
var (
    ErrInvalidAuth          = errors.New("openrouter: invalid auth")
    ErrInsufficientCredits  = errors.New("openrouter: insufficient credits")
    ErrRateLimited          = errors.New("openrouter: rate limited")
    ErrUpstream             = errors.New("openrouter: upstream error")
)

type APIError struct {
    StatusCode int
    Code       string        // OpenRouter's error.code
    Message    string        // OpenRouter's error.message
    RetryAfter time.Duration // populated only for ErrRateLimited
}

func (e *APIError) Error() string
```

### 3.4 `pkg/tgbot/typing.go`

```go
// WithTyping shows "<bot> is typing..." in the originating chat for as long as
// fn runs. Returns fn's result unchanged. Errors from the typing action itself
// are swallowed — they should never fail the caller's actual work.
func WithTyping(c *Context, fn func() error) error
```

Implementation: send `sendChatAction(action="typing")` immediately, spawn a goroutine that re-sends every 4 seconds via a `time.Ticker`, cancel via channel when fn returns. The Telegram indicator visibly lasts ~5 seconds, so a 4-second tick keeps it continuous.

For testability the actual sender is injectable via a private `withTyping(chatID, send, fn)` function — production calls bypass it through `WithTyping` which captures `c.api`.

## 4. Error mapping

| OpenRouter response | Sentinel | `APIError` populated? | Notes |
|---|---|---|---|
| HTTP 200 with parseable body | nil | n/a | success |
| HTTP 401 | `ErrInvalidAuth` | yes | operator must rotate key |
| HTTP 402 | `ErrInsufficientCredits` | yes | account out of credits; service layer alerts ops |
| HTTP 429 | `ErrRateLimited` | yes, `RetryAfter` from `Retry-After` header | caller may backoff |
| HTTP 4xx (other) / 5xx | `ErrUpstream` | yes | generic upstream failure |
| Body not parseable as expected envelope | `ErrUpstream` | yes if status code available, else no | malformed response |
| Network error / DNS / TCP / TLS | `ErrUpstream` | no | raw error chained via `%w` |
| Context cancelled / deadline exceeded | `context.Canceled` / `context.DeadlineExceeded` (passed through, not wrapped) | no | callers detect with `errors.Is(err, context.Canceled)` |

All non-context errors are returned as `errors.Join(sentinel, *APIError-or-raw-err)` so callers can:
- `errors.Is(err, openrouter.ErrRateLimited)` — branch on class
- `var ae *openrouter.APIError; errors.As(err, &ae)` — read status/code/message/RetryAfter

## 5. Data flow

```
Caller (future chat-flow service)
   │
   ▼ ChatRequest{Model, Messages, *Temperature, *TopP, *MaxTokens}
client.Complete(ctx, req)
   ├── marshal JSON (skip nil optional fields)
   ├── POST <baseURL>/chat/completions
   │     headers: Authorization: Bearer <apiKey>
   │              Content-Type: application/json
   │              HTTP-Referer: <appURL>          (if WithApp set)
   │              X-Title: <appName>              (if WithApp set)
   ├── on transport error → return ErrUpstream
   ├── on context cancel → return ctx err
   ├── on non-2xx → parse error envelope, map to sentinel
   └── on 2xx → unmarshal envelope, return ChatResponse
```

## 6. Testing

### 6.1 `pkg/openrouter/client_test.go`

Eight tests, all using `httptest.NewServer` so no real network:

| # | Test name | Server behaviour | Assertion |
|---|---|---|---|
| 1 | `Complete_HappyPath` | 200 + canned envelope | response fields populated correctly |
| 2 | `Complete_RequestShape` | 200; inspect inbound request | Authorization Bearer, HTTP-Referer, X-Title, model, messages, temperature/top_p/max_tokens (or omitted when nil) |
| 3 | `Complete_401_ReturnsErrInvalidAuth` | 401 + error envelope | `errors.Is` + `errors.As` to APIError |
| 4 | `Complete_402_ReturnsErrInsufficientCredits` | 402 | sentinel matches |
| 5 | `Complete_429_ReturnsErrRateLimited` | 429 + `Retry-After: 30` | sentinel + `APIError.RetryAfter == 30s` |
| 6 | `Complete_500_ReturnsErrUpstream` | 500 | sentinel + `APIError.StatusCode == 500` |
| 7 | `Complete_NetworkError_ReturnsErrUpstream` | injected `http.RoundTripper` that always errors | sentinel matches; no APIError |
| 8 | `Complete_ContextCancelled` | server sleeps; ctx pre-cancelled | `errors.Is(err, context.Canceled)` |

### 6.2 `pkg/tgbot/typing_test.go`

Four tests via the unexported `withTyping(chatID, sendActionFunc, fn)`:

| # | Test | Assertion |
|---|---|---|
| 1 | `withTyping_PassesThroughFnResult` | fn returns sentinel → withTyping returns same |
| 2 | `withTyping_SendsImmediately` | first action call observed within 50ms |
| 3 | `withTyping_TicksEvery4s` | fn sleeps 9s → at least 3 calls (t=0, t≈4s, t≈8s) |
| 4 | `withTyping_StopsAfterFnReturns` | no further calls 5s after fn returns |

Test #3 is intentionally slow (~9s). Marked OK in `-race -short` because the test still finishes; we don't add a `-short` skip — the suite as a whole stays under a minute.

## 7. Out of scope (deferred)

- Wiring `Complete` into a chat-flow service that builds the prompt from `chat → character.base_prompt + context.prompt + tg_messages[recent]`. **Next spec.**
- Streaming SSE (`/chat/completions` with `stream: true`).
- Tool / function-calling.
- Image / multi-modal inputs.
- Live integration test against real OpenRouter (deferred behind `//go:build live`).
- Automatic retry on `ErrRateLimited` or `ErrUpstream`. Caller-controlled.
- Multiple completions per request (`n > 1`).
- Logprobs / tool outputs / structured output / response format.
- Cost tracking, rate-limit awareness across calls (use a wrapping middleware later if needed).

## 8. Open items at execution time

- The `Retry-After` header format in OpenRouter responses — usually seconds, sometimes HTTP date. Test #5 uses the seconds form (the only one we've seen). Spec assumes seconds; if date form appears in the wild, extend the parser.
- Whether to set sane defaults for `temperature` / `max_tokens` when the model_config row has them NULL. The client sends nothing in that case (OpenRouter's defaults apply). Service layer can wrap if it wants different defaults.
- App attribution values — `OPENROUTER_APP_URL` defaults to `"https://rollton.com"` (placeholder until the real domain is owned); `OPENROUTER_APP_NAME` defaults to `"Rollton"`. Both are dev-time concerns; correct at execution.
- Whether to gate `OPENROUTER_API_KEY` as required at startup (`config.Load` returns an error if missing). Default: not required — the bot can still long-poll Telegram and serve `/api/v1/me` without OpenRouter. The chat-flow service (next slice) will check at use time.
