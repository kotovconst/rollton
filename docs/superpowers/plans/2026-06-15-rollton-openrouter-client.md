# OpenRouter Client + WithTyping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land `bot/pkg/openrouter` as a tested, decoupled HTTP client for OpenRouter's chat-completion API, plus `pkg/tgbot.WithTyping` for "rolltonchatbot is typing…" UX. No chat-flow wiring — that's the next slice.

**Architecture:** Pure HTTP-over-JSON client using stdlib `net/http`. Sentinel errors per failure class (`ErrInvalidAuth`, `ErrInsufficientCredits`, `ErrRateLimited`, `ErrUpstream`), each joined via `errors.Join` with an `*APIError` carrier so callers can use either `errors.Is` (class) or `errors.As` (specifics). Tests use `httptest.NewServer` — no real network. `WithTyping` is an unexported `withTyping(chatID, sendActionFunc, fn)` for testability with a thin exported `WithTyping(*Context, func() error)` wrapper.

**Tech Stack:** Go stdlib (`net/http`, `encoding/json`, `errors`, `time`); no new dependencies. Existing test deps (`testify/require`) carry over.

**Reference spec:** `docs/superpowers/specs/2026-06-15-rollton-openrouter-client-design.md`.

---

## Pre-flight

- Docker NOT required for any task in this plan.
- `httptest.NewServer` covers all client tests — no real OpenRouter calls.

---

## File Structure

```
bot/
├── internal/config/
│   ├── config.go                       # MODIFIED — add OPENROUTER_API_KEY, OPENROUTER_APP_URL, OPENROUTER_APP_NAME
│   └── config_test.go                  # MODIFIED — env tests for the three new keys
├── .env.example                        # MODIFIED — document the three new keys
├── .env, .env.rolltonchatbot, .env.admin  # MODIFIED — empty stubs for parity
├── pkg/openrouter/                     # NEW PACKAGE
│   ├── types.go                        # CREATED — Role, Message, ChatRequest, ChatResponse
│   ├── errors.go                       # CREATED — sentinels + APIError
│   ├── client.go                       # CREATED — Client, options, New, Complete
│   └── client_test.go                  # CREATED — 8 tests
└── pkg/tgbot/
    ├── typing.go                       # CREATED — WithTyping (exported) + withTyping (unexported)
    └── typing_test.go                  # CREATED — 4 tests
```

---

## Task 1: Config — `OPENROUTER_API_KEY`, `OPENROUTER_APP_URL`, `OPENROUTER_APP_NAME`

**Files:**
- Modify: `bot/internal/config/config.go`, `bot/internal/config/config_test.go`
- Modify: `bot/.env.example`, `bot/.env`, `bot/.env.rolltonchatbot`, `bot/.env.admin`

- [ ] **Step 1: Add the test**

Open `bot/internal/config/config_test.go` and edit `TestLoad_FromEnv` to set + assert the new env vars. After the existing `t.Setenv("CORS_ALLOWED_ORIGINS", ...)` line:

```go
	t.Setenv("OPENROUTER_API_KEY", "or-test-key")
	t.Setenv("OPENROUTER_APP_URL", "https://rollton.com")
	t.Setenv("OPENROUTER_APP_NAME", "Rollton")
```

After the existing `require.Equal(t, []string{...}, cfg.HTTP.AllowedOrigins)`:

```go
	require.Equal(t, "or-test-key", cfg.OpenRouter.APIKey)
	require.Equal(t, "https://rollton.com", cfg.OpenRouter.AppURL)
	require.Equal(t, "Rollton", cfg.OpenRouter.AppName)
```

Add a new test below the existing ones:

```go
func TestLoad_OpenRouter_EmptyByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OPENROUTER_APP_URL", "")
	t.Setenv("OPENROUTER_APP_NAME", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.OpenRouter.APIKey)
	require.Empty(t, cfg.OpenRouter.AppURL)
	require.Empty(t, cfg.OpenRouter.AppName)
}
```

- [ ] **Step 2: Run, expect compile-fail**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go test ./internal/config/... 2>&1 | tail -5
```
Expected: `cfg.OpenRouter undefined`.

- [ ] **Step 3: Update `config.go`**

Open `bot/internal/config/config.go`. Add a new struct type next to `HTTPConfig`:

```go
type OpenRouterConfig struct {
	APIKey  string
	AppURL  string
	AppName string
}
```

Add the field to `Config`:

```go
type Config struct {
	DB            DBConfig
	HTTP          HTTPConfig
	Log           LogConfig
	TelegramToken string
	OpenRouter    OpenRouterConfig
}
```

Inside `Load()`, add to the `cfg :=` literal after the `TelegramToken: ...` line:

```go
		OpenRouter: OpenRouterConfig{
			APIKey:  os.Getenv("OPENROUTER_API_KEY"),
			AppURL:  os.Getenv("OPENROUTER_APP_URL"),
			AppName: os.Getenv("OPENROUTER_APP_NAME"),
		},
```

No validation: an empty `APIKey` is permitted at config time (the chat-flow service in a later slice will require it). Section 8 of the spec made this explicit.

- [ ] **Step 4: Tests pass**

```bash
go test ./internal/config/... -v 2>&1 | tail -10
```
Expected: all PASS.

- [ ] **Step 5: Document in `bot/.env.example`**

Open `bot/.env.example`. Append:

```dotenv

# OpenRouter LLM client. APIKey required at chat-flow-use time; missing key is
# tolerated at startup so the bot can still long-poll Telegram and serve /me.
# AppURL + AppName are sent as HTTP-Referer + X-Title for OpenRouter usage
# attribution; safe to leave empty.
OPENROUTER_API_KEY=
OPENROUTER_APP_URL=https://rollton.com
OPENROUTER_APP_NAME=Rollton
```

- [ ] **Step 6: Stub in the other env files (parity)**

Append the same three lines (`OPENROUTER_API_KEY=`, `OPENROUTER_APP_URL=`, `OPENROUTER_APP_NAME=`) to each of:

```
bot/.env
bot/.env.rolltonchatbot
bot/.env.admin
```

The two non-`.env.example` files are gitignored — commit only touches `.env.example`.

- [ ] **Step 7: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/config/ bot/.env.example
git commit -m "feat(bot): config — OpenRouterConfig from OPENROUTER_* env vars"
```

---

## Task 2: `pkg/openrouter/types.go`

**Files:**
- Create: `bot/pkg/openrouter/types.go`

- [ ] **Step 1: Create the file**

```go
package openrouter

// Role identifies the speaker for an LLM message.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a chat conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the input to Client.Complete.
//
// Temperature, TopP, and MaxTokens are pointer fields so nil = "use OpenRouter
// default". This maps cleanly to the NULL columns in model_configs.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature *float64
	TopP        *float64
	MaxTokens   *int
}

// ChatResponse is the result of a successful Complete call.
//
// Reply is the assistant message text (choices[0].message.content).
// FinishReason is OpenRouter's: "stop" | "length" | "content_filter" | ...
type ChatResponse struct {
	Model        string
	Reply        string
	TokensIn     int
	TokensOut    int
	FinishReason string
}
```

- [ ] **Step 2: Build**

```bash
go build ./pkg/openrouter/...
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/openrouter/types.go
git commit -m "feat(openrouter): types.go — Role, Message, ChatRequest, ChatResponse"
```

---

## Task 3: `pkg/openrouter/errors.go`

**Files:**
- Create: `bot/pkg/openrouter/errors.go`

- [ ] **Step 1: Create the file**

```go
package openrouter

import (
	"errors"
	"fmt"
	"time"
)

// Sentinel errors. Compose with *APIError via errors.Join, so callers can use
// either errors.Is(err, ErrX) for class-level branching or errors.As(err, &APIError{})
// to read status/code/message/RetryAfter.
var (
	// ErrInvalidAuth: HTTP 401. Operator must rotate the key.
	ErrInvalidAuth = errors.New("openrouter: invalid auth")

	// ErrInsufficientCredits: HTTP 402. Account out of credits;
	// service layer should alert ops + return "service unavailable" to user.
	ErrInsufficientCredits = errors.New("openrouter: insufficient credits")

	// ErrRateLimited: HTTP 429. Caller may back off using APIError.RetryAfter.
	ErrRateLimited = errors.New("openrouter: rate limited")

	// ErrUpstream: any other 4xx/5xx, malformed response, or transport error.
	ErrUpstream = errors.New("openrouter: upstream error")
)

// APIError carries the OpenRouter error envelope and (when known) HTTP status.
// Always joined with a sentinel via errors.Join — never returned alone.
type APIError struct {
	StatusCode int
	Code       string        // OpenRouter's error.code (may be empty)
	Message    string        // OpenRouter's error.message (may be empty)
	RetryAfter time.Duration // populated only when ErrRateLimited
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("openrouter: status=%d code=%q message=%q", e.StatusCode, e.Code, e.Message)
	}
	return fmt.Sprintf("openrouter: status=%d", e.StatusCode)
}
```

- [ ] **Step 2: Build**

```bash
go build ./pkg/openrouter/...
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/openrouter/errors.go
git commit -m "feat(openrouter): errors.go — typed sentinels + APIError carrier"
```

---

## Task 4: `pkg/openrouter/client.go` (skeleton — no Complete yet)

**Files:**
- Create: `bot/pkg/openrouter/client.go`

- [ ] **Step 1: Create the file**

```go
// Package openrouter is a minimal HTTP client for the OpenRouter
// chat-completion API. See docs/superpowers/specs/2026-06-15-rollton-openrouter-client-design.md.
package openrouter

import (
	"net/http"
)

// defaultBaseURL is the production OpenRouter API endpoint.
const defaultBaseURL = "https://openrouter.ai/api/v1"

// Client is a configured OpenRouter HTTP client. Use New + options to construct.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	appURL     string // sent as HTTP-Referer if non-empty
	appName    string // sent as X-Title if non-empty
}

// Option mutates a Client during construction.
type Option func(*Client)

// WithHTTPClient swaps the underlying HTTP client. Use in tests + for custom
// timeouts. Default: http.DefaultClient.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithBaseURL overrides the API base URL. Use in tests against httptest.NewServer.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithApp sets the HTTP-Referer + X-Title headers OpenRouter uses for usage
// attribution. Both required for the headers to be sent; empty disables.
func WithApp(url, name string) Option {
	return func(c *Client) {
		c.appURL = url
		c.appName = name
	}
}

// New constructs a Client with the provided API key.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}
```

- [ ] **Step 2: Build**

```bash
go build ./pkg/openrouter/...
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/openrouter/client.go
git commit -m "feat(openrouter): client.go skeleton — Client, options, New"
```

---

## Task 5: `pkg/openrouter/client_test.go` — happy path + request shape (TDD)

**Files:**
- Create: `bot/pkg/openrouter/client_test.go`

- [ ] **Step 1: Write the failing tests**

Create `bot/pkg/openrouter/client_test.go`:

```go
package openrouter_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/pkg/openrouter"
)

// canned successful response — mirrors OpenRouter's actual shape.
const cannedSuccess = `{
  "id": "gen-test",
  "model": "anthropic/claude-haiku-4.5",
  "choices": [{
    "message": {"role": "assistant", "content": "hello world"},
    "finish_reason": "stop"
  }],
  "usage": {"prompt_tokens": 10, "completion_tokens": 5}
}`

func TestComplete_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))
	res, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model:    "anthropic/claude-haiku-4.5",
		Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	require.Equal(t, "anthropic/claude-haiku-4.5", res.Model)
	require.Equal(t, "hello world", res.Reply)
	require.Equal(t, 10, res.TokensIn)
	require.Equal(t, 5, res.TokensOut)
	require.Equal(t, "stop", res.FinishReason)
}

func TestComplete_RequestShape(t *testing.T) {
	temp := 0.7
	topP := 0.9
	maxT := 256

	var (
		gotAuth     string
		gotReferer  string
		gotTitle    string
		gotBody     map[string]any
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("test-key",
		openrouter.WithBaseURL(srv.URL),
		openrouter.WithApp("https://rollton.com", "Rollton"),
	)
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "anthropic/claude-haiku-4.5",
		Messages: []openrouter.Message{
			{Role: openrouter.RoleSystem, Content: "you are alice"},
			{Role: openrouter.RoleUser, Content: "hi"},
		},
		Temperature: &temp,
		TopP:        &topP,
		MaxTokens:   &maxT,
	})

	require.NoError(t, err)
	require.Equal(t, "Bearer test-key", gotAuth)
	require.Equal(t, "https://rollton.com", gotReferer)
	require.Equal(t, "Rollton", gotTitle)
	require.Equal(t, "anthropic/claude-haiku-4.5", gotBody["model"])
	require.InDelta(t, 0.7, gotBody["temperature"], 0.0001)
	require.InDelta(t, 0.9, gotBody["top_p"], 0.0001)
	require.Equal(t, float64(256), gotBody["max_tokens"])

	msgs, ok := gotBody["messages"].([]any)
	require.True(t, ok)
	require.Len(t, msgs, 2)
	require.Equal(t, "system", msgs[0].(map[string]any)["role"])
	require.Equal(t, "you are alice", msgs[0].(map[string]any)["content"])
}

func TestComplete_RequestShape_OmitsNilOptionals(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("test-key", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model:    "anthropic/claude-haiku-4.5",
		Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	_, hasTemp := gotBody["temperature"]
	_, hasTopP := gotBody["top_p"]
	_, hasMaxT := gotBody["max_tokens"]
	require.False(t, hasTemp, "temperature should be omitted when nil")
	require.False(t, hasTopP, "top_p should be omitted when nil")
	require.False(t, hasMaxT, "max_tokens should be omitted when nil")
}
```

- [ ] **Step 2: Run, expect failure**

```bash
go test ./pkg/openrouter/... 2>&1 | tail -5
```
Expected: `Complete undefined`.

- [ ] **Step 3: Implement `Complete`**

**Replace `bot/pkg/openrouter/client.go` entirely** with the following (the skeleton's content from Task 4 is included so this is a single coherent file, not two blocks):

```go
// Package openrouter is a minimal HTTP client for the OpenRouter
// chat-completion API. See docs/superpowers/specs/2026-06-15-rollton-openrouter-client-design.md.
package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// defaultBaseURL is the production OpenRouter API endpoint.
const defaultBaseURL = "https://openrouter.ai/api/v1"

// Client is a configured OpenRouter HTTP client. Use New + options to construct.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
	appURL     string // sent as HTTP-Referer if non-empty
	appName    string // sent as X-Title if non-empty
}

// Option mutates a Client during construction.
type Option func(*Client)

// WithHTTPClient swaps the underlying HTTP client. Use in tests + for custom timeouts.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.httpClient = h } }

// WithBaseURL overrides the API base URL. Use in tests against httptest.NewServer.
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithApp sets the HTTP-Referer + X-Title headers OpenRouter uses for usage attribution.
func WithApp(url, name string) Option {
	return func(c *Client) {
		c.appURL = url
		c.appName = name
	}
}

// New constructs a Client with the provided API key.
func New(apiKey string, opts ...Option) *Client {
	c := &Client{
		apiKey:     apiKey,
		httpClient: http.DefaultClient,
		baseURL:    defaultBaseURL,
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// wire types (private; mirror OpenRouter's JSON shape exactly).

type wireRequest struct {
	Model       string         `json:"model"`
	Messages    []wireMessage  `json:"messages"`
	Temperature *float64       `json:"temperature,omitempty"`
	TopP        *float64       `json:"top_p,omitempty"`
	MaxTokens   *int           `json:"max_tokens,omitempty"`
}

type wireMessage struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type wireResponse struct {
	ID      string         `json:"id"`
	Model   string         `json:"model"`
	Choices []wireChoice   `json:"choices"`
	Usage   wireUsage      `json:"usage"`
	Error   *wireError     `json:"error,omitempty"`
}

type wireChoice struct {
	Message      wireMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type wireUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorEnvelope struct {
	Error wireError `json:"error"`
}

// Complete sends a single chat-completion request.
func (c *Client) Complete(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	body := wireRequest{
		Model:       req.Model,
		Messages:    make([]wireMessage, 0, len(req.Messages)),
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   req.MaxTokens,
	}
	for _, m := range req.Messages {
		body.Messages = append(body.Messages, wireMessage{Role: m.Role, Content: m.Content})
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return ChatResponse{}, errors.Join(ErrUpstream, fmt.Errorf("marshal: %w", err))
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return ChatResponse{}, errors.Join(ErrUpstream, fmt.Errorf("build request: %w", err))
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	if c.appURL != "" {
		httpReq.Header.Set("HTTP-Referer", c.appURL)
	}
	if c.appName != "" {
		httpReq.Header.Set("X-Title", c.appName)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		// Context errors pass through (caller uses errors.Is(err, context.Canceled)).
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ChatResponse{}, ctxErr
		}
		return ChatResponse{}, errors.Join(ErrUpstream, err)
	}
	defer resp.Body.Close()
	rawBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return ChatResponse{}, mapHTTPError(resp.StatusCode, resp.Header, rawBody)
	}

	var wr wireResponse
	if err := json.Unmarshal(rawBody, &wr); err != nil {
		return ChatResponse{}, errors.Join(ErrUpstream, &APIError{StatusCode: resp.StatusCode, Message: "malformed response body"})
	}
	if len(wr.Choices) == 0 {
		return ChatResponse{}, errors.Join(ErrUpstream, &APIError{StatusCode: resp.StatusCode, Message: "no choices in response"})
	}
	return ChatResponse{
		Model:        wr.Model,
		Reply:        wr.Choices[0].Message.Content,
		TokensIn:     wr.Usage.PromptTokens,
		TokensOut:    wr.Usage.CompletionTokens,
		FinishReason: wr.Choices[0].FinishReason,
	}, nil
}

func mapHTTPError(status int, hdr http.Header, body []byte) error {
	apiErr := &APIError{StatusCode: status}
	var env errorEnvelope
	if json.Unmarshal(body, &env) == nil {
		apiErr.Code = env.Error.Code
		apiErr.Message = env.Error.Message
	}

	switch status {
	case http.StatusUnauthorized:
		return errors.Join(ErrInvalidAuth, apiErr)
	case http.StatusPaymentRequired:
		return errors.Join(ErrInsufficientCredits, apiErr)
	case http.StatusTooManyRequests:
		if ra := hdr.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil {
				apiErr.RetryAfter = time.Duration(secs) * time.Second
			}
		}
		return errors.Join(ErrRateLimited, apiErr)
	default:
		return errors.Join(ErrUpstream, apiErr)
	}
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
go test ./pkg/openrouter/... -v 2>&1 | tail -15
```
Expected: 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/openrouter/
git commit -m "feat(openrouter): Complete — POST /chat/completions, decode, error mapping"
```

---

## Task 6: Error-path tests

**Files:**
- Modify: `bot/pkg/openrouter/client_test.go`

- [ ] **Step 1: Append the error tests**

Add to the end of `bot/pkg/openrouter/client_test.go`:

```go
const cannedError401 = `{"error": {"code": "invalid_api_key", "message": "Invalid API key"}}`
const cannedError402 = `{"error": {"code": "insufficient_credits", "message": "Out of credits"}}`
const cannedError429 = `{"error": {"code": "rate_limited", "message": "Slow down"}}`
const cannedError500 = `{"error": {"code": "server_error", "message": "Internal error"}}`

func TestComplete_401_ReturnsErrInvalidAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, cannedError401)
	}))
	defer srv.Close()

	c := openrouter.New("bad", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model:    "x",
		Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})

	require.ErrorIs(t, err, openrouter.ErrInvalidAuth)
	var apiErr *openrouter.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 401, apiErr.StatusCode)
	require.Equal(t, "invalid_api_key", apiErr.Code)
	require.Contains(t, apiErr.Message, "Invalid API key")
}

func TestComplete_402_ReturnsErrInsufficientCredits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = io.WriteString(w, cannedError402)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrInsufficientCredits)
}

func TestComplete_429_ReturnsErrRateLimited_WithRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "30")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, cannedError429)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrRateLimited)
	var apiErr *openrouter.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 30*time.Second, apiErr.RetryAfter)
}

func TestComplete_500_ReturnsErrUpstream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, cannedError500)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrUpstream)
	var apiErr *openrouter.APIError
	require.ErrorAs(t, err, &apiErr)
	require.Equal(t, 500, apiErr.StatusCode)
}

type erroringRoundTripper struct{}

func (erroringRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("simulated network error")
}

func TestComplete_NetworkError_ReturnsErrUpstream(t *testing.T) {
	c := openrouter.New("k",
		openrouter.WithBaseURL("http://localhost:1"),
		openrouter.WithHTTPClient(&http.Client{Transport: erroringRoundTripper{}}),
	)
	_, err := c.Complete(context.Background(), openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, openrouter.ErrUpstream)
}

func TestComplete_ContextCancelled(t *testing.T) {
	// Server sleeps so the client must wait on the cancelled ctx.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, cannedSuccess)
	}))
	defer srv.Close()

	c := openrouter.New("k", openrouter.WithBaseURL(srv.URL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Complete(ctx, openrouter.ChatRequest{
		Model: "x", Messages: []openrouter.Message{{Role: openrouter.RoleUser, Content: "hi"}},
	})
	require.ErrorIs(t, err, context.Canceled)
}
```

Add these imports at the top of the file if they're not present already: `"errors"`, `"time"`.

- [ ] **Step 2: Run all tests**

```bash
go test ./pkg/openrouter/... -v 2>&1 | tail -25
```
Expected: 8 tests PASS.

- [ ] **Step 3: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/openrouter/client_test.go
git commit -m "test(openrouter): error-path tests — 401/402/429/500/network/ctx-cancel"
```

---

## Task 7: `pkg/tgbot/typing.go` + tests (TDD)

**Files:**
- Create: `bot/pkg/tgbot/typing.go`
- Create: `bot/pkg/tgbot/typing_test.go`

- [ ] **Step 1: Write the failing tests**

Create `bot/pkg/tgbot/typing_test.go`:

```go
package tgbot

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithTyping_PassesThroughFnResult(t *testing.T) {
	sentinel := errors.New("from fn")
	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	err := withTyping(123, send, func() error { return sentinel })

	require.ErrorIs(t, err, sentinel)
	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(1))
}

func TestWithTyping_SendsImmediately(t *testing.T) {
	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	_ = withTyping(123, send, func() error {
		// Even with no-op fn, at least the initial send must have happened.
		return nil
	})

	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(1))
}

func TestWithTyping_TicksDuringLongFn(t *testing.T) {
	// Override the tick interval for fast tests.
	prev := tickInterval
	tickInterval = 50 * time.Millisecond
	t.Cleanup(func() { tickInterval = prev })

	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	_ = withTyping(123, send, func() error {
		time.Sleep(180 * time.Millisecond) // ~3-4 ticks
		return nil
	})

	require.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
}

func TestWithTyping_StopsAfterFnReturns(t *testing.T) {
	prev := tickInterval
	tickInterval = 50 * time.Millisecond
	t.Cleanup(func() { tickInterval = prev })

	var calls int32
	send := func(_ int64) error { atomic.AddInt32(&calls, 1); return nil }

	require.NoError(t, withTyping(123, send, func() error { return nil }))
	before := atomic.LoadInt32(&calls)
	time.Sleep(200 * time.Millisecond)
	after := atomic.LoadInt32(&calls)

	require.Equal(t, before, after, "no additional sends after fn returned")
}

func TestWithTyping_SendErrorsAreSwallowed(t *testing.T) {
	send := func(_ int64) error { return errors.New("send failed") }

	require.NoError(t, withTyping(123, send, func() error { return nil }))
}
```

- [ ] **Step 2: Run, expect failure**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go test ./pkg/tgbot/... -run Typing 2>&1 | tail -5
```
Expected: `withTyping undefined` and `tickInterval undefined`.

- [ ] **Step 3: Write `typing.go`**

Create `bot/pkg/tgbot/typing.go`:

```go
package tgbot

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// tickInterval is how often we re-send the typing action. Telegram's typing
// indicator visibly lasts ~5 seconds, so 4 seconds keeps it continuous.
// Test override: lower this value for fast tests via t.Cleanup-restored mutation.
var tickInterval = 4 * time.Second

// sendActionFunc lets the typing loop call Telegram without depending on the
// concrete *tgbotapi.BotAPI. Tests inject a recording fake; production code
// uses defaultSendAction.
type sendActionFunc func(chatID int64) error

// WithTyping shows "rolltonchatbot is typing..." in the chat for as long as fn
// runs. Returns fn's result unchanged. Errors from the typing action itself
// are swallowed — they should never fail the caller's actual work.
//
// No chat ID = no-op (we silently run fn and return).
func WithTyping(c *Context, fn func() error) error {
	chatID := c.ChatID()
	if chatID == 0 {
		return fn()
	}
	return withTyping(chatID, defaultSendAction(c.api), fn)
}

func defaultSendAction(api *tgbotapi.BotAPI) sendActionFunc {
	if api == nil {
		return func(int64) error { return nil }
	}
	return func(chatID int64) error {
		_, err := api.Request(tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping))
		return err
	}
}

// withTyping is the testable core. send is called immediately + every
// tickInterval until fn returns.
func withTyping(chatID int64, send sendActionFunc, fn func() error) error {
	// Initial send (swallow errors).
	_ = send(chatID)

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				_ = send(chatID)
			}
		}
	}()

	err := fn()
	close(stop)
	<-done
	return err
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
go test -race -short ./pkg/tgbot/... -v 2>&1 | tail -15
```
Expected: 5 Typing tests PASS (plus the existing tgbot router tests).

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/pkg/tgbot/typing.go bot/pkg/tgbot/typing_test.go
git commit -m "feat(tgbot): WithTyping — shows 'is typing...' indicator while fn runs"
```

---

## Task 8: Full verification + push

- [ ] **Step 1: Full unit-test sweep**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
make test 2>&1 | tail -10
```
Expected: all PASS (existing + 8 openrouter + 5 typing = 13 new tests).

- [ ] **Step 2: Build**

```bash
make build 2>&1 | tail -5
```
Expected: `bin/admin`, `bin/rolltonchatbot` built.

- [ ] **Step 3: Lint**

```bash
make lint 2>&1 | tail -3
```
Expected: no output, exit 0.

- [ ] **Step 4: Tree clean + push**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git status
git push origin main
```
Expected: clean tree, push succeeds.

---

## Out-of-scope (explicit non-goals)

- Wiring `Complete` into a service that talks to the database. **Next spec.**
- Streaming SSE responses.
- Tool / function-calling completions.
- Image / audio / multi-modal input.
- Live integration test against real OpenRouter (deferred behind `//go:build live`).
- Automatic retry on `ErrRateLimited` / `ErrUpstream`.
- Multiple completions per request (`n > 1`).
- Logprobs, structured-output schemas, response format constraints.
- Per-request cost tracking, rate-limit awareness across calls.

## Open items at execution time

- `Retry-After` header form: spec assumes seconds. If OpenRouter ever sends an HTTP date instead, Task 6's `TestComplete_429_ReturnsErrRateLimited_WithRetryAfter` will silently set `RetryAfter = 0` — extend `mapHTTPError` to handle that form.
- `OPENROUTER_APP_URL` placeholder: `https://rollton.com` is used in `.env.example`. Replace when a real domain is registered.
- `OPENROUTER_APP_NAME`: `"Rollton"` placeholder. Fine to keep.
- Default HTTP timeout: client uses `http.DefaultClient` (no timeout). Production callers should wrap with `context.WithTimeout` — to be enforced when the chat-flow service lands.
