# TMA Login (initData → /api/v1/me) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the auth loop between the mini-app and the bot. After this plan: open the mini-app from `@rolltonchatbot` → no interaction → SPA shows "Hi, <first_name>" backed by an HMAC-verified server identity and a row in `users`.

**Architecture:** New HTTP middleware `TmaAuth` validates `Authorization: tma <initDataRaw>` via `github.com/telegram-mini-apps/init-data-golang`, calls the existing `UserService.EnsureRegistered`, puts `*domain.User` into request context. New `GET /api/v1/me` handler reads from ctx and returns `{user, settings(defaults)}`. CORS middleware wraps the mux. Frontend gets a `useMe()` hook + `AuthBoundary` gate in Layout + a tiny `WelcomeHeader`.

**Tech Stack:** Go (existing); `github.com/telegram-mini-apps/init-data-golang`; React 18 + TanStack Query (existing); MSW for frontend tests (existing); pgxmock NOT needed (auth middleware uses a fake `UserService`, not the DB).

**Reference spec:** `docs/superpowers/specs/2026-06-13-rollton-tma-login-design.md`.

---

## Pre-flight

- Docker daemon NOT required for any task in this plan.
- All tests are unit tests (HTTP via `httptest`, frontend via `vitest` + MSW).

---

## File Structure

```
bot/
├── go.mod  go.sum                                    # MODIFIED — add init-data-golang
├── internal/
│   ├── config/config.go                              # MODIFIED — add HTTPConfig.AllowedOrigins
│   ├── config/config_test.go                         # MODIFIED — add origin tests
│   ├── middleware/
│   │   ├── tma_auth.go                               # CREATED
│   │   ├── cors.go                                   # CREATED
│   │   └── test/
│   │       ├── tma_auth_test.go                      # CREATED
│   │       └── cors_test.go                          # CREATED
│   └── bots/rolltonchatbot/
│       ├── wire.go                                   # MODIFIED — wire /api/v1/me + CORS
│       └── handlers/http/
│           └── me_handler.go                         # CREATED

web/
├── src/
│   ├── api/user.ts                                   # MODIFIED — adjust me() return type slightly
│   ├── hooks/useMe.ts                                # CREATED
│   ├── components/
│   │   ├── AuthBoundary.tsx                          # CREATED
│   │   └── Layout.tsx                                # MODIFIED — wrap routes in AuthBoundary + WelcomeHeader
│   └── test/
│       ├── setup.tsx                                 # MODIFIED — default MSW handler for /api/v1/me
│       ├── AuthBoundary.test.tsx                     # CREATED
│       └── HomePage.test.tsx                         # MODIFIED — survives new Layout gating
```

---

## Task 1: Add `init-data-golang` dependency

**Files:**
- Modify: `bot/go.mod`, `bot/go.sum`

- [ ] **Step 1: Install**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
go get github.com/telegram-mini-apps/init-data-golang@latest
```
Expected: dependency added; `go.sum` updated.

- [ ] **Step 2: Verify build still passes**

```bash
go build ./...
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/go.mod bot/go.sum
git commit -m "chore(bot): add init-data-golang for TMA initData validation"
```

---

## Task 2: Config — `HTTPConfig.AllowedOrigins`

**Files:**
- Modify: `bot/internal/config/config.go`
- Modify: `bot/internal/config/config_test.go`
- Modify: `bot/.env.example`, `bot/.env`, `bot/.env.rolltonchatbot`, `bot/.env.admin`

- [ ] **Step 1: Update `config_test.go` (TDD)**

Open `bot/internal/config/config_test.go`. Add this test inside the existing `TestLoad_FromEnv`:

```go
t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.rollton.com,https://example.dev")
```

Inside the existing assertions block (after the existing `require.Equal` calls), add:

```go
require.Equal(t, []string{"https://app.rollton.com", "https://example.dev"}, cfg.HTTP.AllowedOrigins)
```

Add a new test below the existing tests:

```go
func TestLoad_AllowedOrigins_EmptyByDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Empty(t, cfg.HTTP.AllowedOrigins)
}

func TestLoad_AllowedOrigins_TrimsWhitespace(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("TELEGRAM_TOKEN", "abc")
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://a.com , https://b.com ")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.Equal(t, []string{"https://a.com", "https://b.com"}, cfg.HTTP.AllowedOrigins)
}
```

- [ ] **Step 2: Run, expect failures**

```bash
go test ./internal/config/... -v 2>&1 | tail -10
```
Expected: 2-3 failures referencing `cfg.HTTP.AllowedOrigins` (field doesn't exist yet).

- [ ] **Step 3: Update `config.go`**

In `bot/internal/config/config.go`:

```go
type HTTPConfig struct {
	Port           int
	AllowedOrigins []string
}
```

Add a helper function (place near `getEnvInt`):

```go
func getEnvList(key string) []string {
	raw := os.Getenv(key)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
```

Add `"strings"` to imports if not present.

In `Load()`, update the `HTTP` field assignment:

```go
HTTP: HTTPConfig{
	Port:           getEnvInt("HTTP_PORT", 8080),
	AllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS"),
},
```

- [ ] **Step 4: Tests pass**

```bash
go test ./internal/config/... -v 2>&1 | tail -15
```
Expected: all PASS.

- [ ] **Step 5: Document the new env var**

Append to `bot/.env.example`:

```dotenv

# Comma-separated origins allowed to call /api/v1/*. Empty = cross-origin denied.
CORS_ALLOWED_ORIGINS=
```

Append the same `CORS_ALLOWED_ORIGINS=` line to `bot/.env`, `bot/.env.rolltonchatbot`, `bot/.env.admin` (only the rolltonchatbot one actually serves /api/v1/, but admin keeps the parsing parity).

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/config/ bot/.env.example bot/.env.rolltonchatbot bot/.env.admin
git commit -m "feat(bot): config — HTTPConfig.AllowedOrigins from CORS_ALLOWED_ORIGINS"
```

(`bot/.env` is gitignored — won't be committed.)

---

## Task 3: CORS middleware (TDD)

**Files:**
- Create: `bot/internal/middleware/cors.go`
- Create: `bot/internal/middleware/test/cors_test.go`

- [ ] **Step 1: Write the failing test**

Create `bot/internal/middleware/test/cors_test.go`:

```go
package test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kotovconst/rollton/bot/internal/middleware"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestCORS_AllowedOrigin_EchoesHeader(t *testing.T) {
	h := middleware.CORS([]string{"https://app.rollton.com"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://app.rollton.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "https://app.rollton.com", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Equal(t, "Vary", rec.Header().Get("Vary")[:4])
}

func TestCORS_DisallowedOrigin_OmitsHeader(t *testing.T) {
	h := middleware.CORS([]string{"https://app.rollton.com"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_Preflight_Returns204WithAllowHeaders(t *testing.T) {
	h := middleware.CORS([]string{"https://app.rollton.com"})(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/me", nil)
	req.Header.Set("Origin", "https://app.rollton.com")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)
	require.Equal(t, "https://app.rollton.com", rec.Header().Get("Access-Control-Allow-Origin"))
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
	require.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), "Authorization")
}

func TestCORS_NoOriginHeader_NoChanges(t *testing.T) {
	h := middleware.CORS([]string{"https://app.rollton.com"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}
```

- [ ] **Step 2: Run, expect compile fail**

```bash
go test ./internal/middleware/test/... -run CORS 2>&1 | tail -5
```
Expected: `middleware.CORS undefined`.

- [ ] **Step 3: Write `bot/internal/middleware/cors.go`**

```go
package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that allows cross-origin requests from origins in
// the provided allowlist. Empty allowlist = no cross-origin access.
//
// Echoes the request's Origin header back in Access-Control-Allow-Origin when
// allowed. Adds Vary: Origin so caches don't pollute across origins. Handles
// OPTIONS preflight directly (returns 204 + allowed methods/headers).
func CORS(allowed []string) func(http.Handler) http.Handler {
	set := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		set[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin == "" {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := set[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")
			}
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(
					[]string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions}, ", "))
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "600")
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
go test ./internal/middleware/test/... -run CORS -v 2>&1 | tail -15
```
Expected: 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/middleware/cors.go bot/internal/middleware/test/cors_test.go
git commit -m "feat(bot): CORS middleware for /api/v1/* with origin allowlist"
```

---

## Task 4: TMA auth middleware (TDD)

**Files:**
- Create: `bot/internal/middleware/tma_auth.go`
- Create: `bot/internal/middleware/test/tma_auth_test.go`

- [ ] **Step 1: Write the failing test**

Create `bot/internal/middleware/test/tma_auth_test.go`:

```go
package test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	initdata "github.com/telegram-mini-apps/init-data-golang"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	"github.com/kotovconst/rollton/bot/internal/middleware"
)

const testBotToken = "1234567890:TESTtoken"

func validInitData(t *testing.T, token string, tgID int64, authDate time.Time) string {
	t.Helper()
	userJSON := fmt.Sprintf(`{"id":%d,"first_name":"Alice","username":"alice","language_code":"en","is_premium":false}`, tgID)
	v := url.Values{}
	v.Set("query_id", "AAH123")
	v.Set("user", userJSON)
	v.Set("auth_date", strconv.FormatInt(authDate.Unix(), 10))
	return initdata.Sign(v.Encode(), token, authDate)
}

func nextOK(captured **domain.User) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, _ := middleware.UserFromContext(r.Context())
		*captured = u
		w.WriteHeader(http.StatusOK)
	})
}

func TestTmaAuth_Valid_PutsUserInCtx_And_Calls_Service(t *testing.T) {
	stored := domain.User{ID: uuid.New(), TelegramID: 42, FirstName: "Alice"}
	svc := &fakeUserSvc{user: stored}
	var captured *domain.User
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(nextOK(&captured))

	raw := validInitData(t, testBotToken, 42, time.Now())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, svc.called)
	require.NotNil(t, captured)
	require.Equal(t, stored.ID, captured.ID)
}

func TestTmaAuth_MissingHeader_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "UNAUTHORIZED")
	require.False(t, svc.called)
}

func TestTmaAuth_WrongPrefix_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "Bearer xyz")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, svc.called)
}

func TestTmaAuth_BadHMAC_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next should not be called")
	}))

	// Signed with a DIFFERENT token.
	raw := validInitData(t, "9876543210:OTHERtoken", 42, time.Now())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, svc.called)
}

func TestTmaAuth_StaleAuthDate_Returns401(t *testing.T) {
	svc := &fakeUserSvc{}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 1*time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next should not be called")
	}))

	raw := validInitData(t, testBotToken, 42, time.Now().Add(-2*time.Hour))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.False(t, svc.called)
}

func TestTmaAuth_ServiceError_Returns500(t *testing.T) {
	svc := &fakeUserSvc{err: errSvcDown}
	h := middleware.TmaAuth(testBotToken, svc, quietLog, 24*time.Hour)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("next should not be called")
	}))

	raw := validInitData(t, testBotToken, 42, time.Now())
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	req.Header.Set("Authorization", "tma "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.True(t, svc.called)
}
```

Add to the existing `auth_test.go` (or a new helpers file) — but the existing `quietLog` is private to that test file. Reuse pattern: copy the line.

Open `bot/internal/middleware/test/auth_test.go` and locate the `quietLog` declaration. Move it to a new file `bot/internal/middleware/test/helpers_test.go` so both test files share it:

```go
package test

import (
	"errors"
	"log/slog"
	"os"
)

var quietLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError + 1}))

var errSvcDown = errors.New("svc down")
```

Remove the original `quietLog` declaration from `auth_test.go`.

- [ ] **Step 2: Run, expect compile fail**

```bash
go test ./internal/middleware/test/... -run TmaAuth 2>&1 | tail -5
```
Expected: `middleware.TmaAuth undefined`.

- [ ] **Step 3: Write `bot/internal/middleware/tma_auth.go`**

```go
package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	initdata "github.com/telegram-mini-apps/init-data-golang"

	"github.com/kotovconst/rollton/bot/internal/core/domain"
	httpx "github.com/kotovconst/rollton/bot/pkg/http"
)

// TmaAuth gates /api/v1/* with Telegram Mini App initData HMAC validation.
// Reads "Authorization: tma <initDataRaw>", verifies via init-data-golang,
// calls svc.EnsureRegistered to materialize the user, and puts *domain.User
// into the request context for downstream handlers.
//
// Any HMAC/parse/freshness failure produces 401 UNAUTHORIZED. The body does
// not distinguish (security best practice — don't help triangulation).
func TmaAuth(botToken string, svc UserService, log *slog.Logger, maxAge time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := parseTmaHeader(r.Header.Get("Authorization"))
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "missing tma credentials")
				return
			}
			if err := initdata.Validate(raw, botToken, maxAge); err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "invalid init data")
				return
			}
			parsed, err := initdata.Parse(raw)
			if err != nil || parsed.User == nil {
				httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "malformed init data")
				return
			}
			user := domain.NewUser(
				parsed.User.ID,
				parsed.User.Username,
				parsed.User.FirstName,
				parsed.User.LastName,
				parsed.User.LanguageCode,
				parsed.User.IsPremium,
			)
			u, err := svc.EnsureRegistered(r.Context(), user)
			if err != nil {
				log.Error("http_user_register_failed", "telegram_id", parsed.User.ID, "err", err)
				httpx.WriteError(w, http.StatusInternalServerError, httpx.ErrCodeInternalError, "registration failed")
				return
			}
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), &u)))
		})
	}
}

func parseTmaHeader(h string) (string, bool) {
	const prefix = "tma "
	if len(h) <= len(prefix) || !strings.HasPrefix(h, prefix) {
		return "", false
	}
	return h[len(prefix):], true
}
```

- [ ] **Step 4: Run, expect PASS**

```bash
go test ./internal/middleware/test/... -run TmaAuth -v 2>&1 | tail -20
```
Expected: 6 tests PASS.

- [ ] **Step 5: Run the whole middleware test suite (regression check)**

```bash
go test -race -short ./internal/middleware/... -v 2>&1 | tail -15
```
Expected: all PASS (the 3 EnsureUserRegistered tests + 4 Allowlist tests + 4 CORS tests + 6 TmaAuth tests = 17).

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/middleware/tma_auth.go bot/internal/middleware/test/
git commit -m "feat(bot): TmaAuth HTTP middleware — validates initData, registers user, puts in ctx"
```

---

## Task 5: `MeHandler` — `/api/v1/me`

**Files:**
- Create: `bot/internal/bots/rolltonchatbot/handlers/http/me_handler.go`

- [ ] **Step 1: Write `me_handler.go`**

```go
package http

import (
	"net/http"

	"github.com/kotovconst/rollton/bot/internal/middleware"
	httpx "github.com/kotovconst/rollton/bot/pkg/http"
)

type MeHandler struct{}

func NewMeHandler() *MeHandler { return &MeHandler{} }

type meResponse struct {
	User     userDTO     `json:"user"`
	Settings settingsDTO `json:"settings"`
}

type userDTO struct {
	ID         string `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username,omitempty"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name,omitempty"`
}

type settingsDTO struct {
	NotificationsEnabled bool   `json:"notifications_enabled"`
	PreferredLanguage    string `json:"preferred_language"`
}

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok || u == nil {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "no user in context")
		return
	}
	httpx.WriteSuccess(w, meResponse{
		User: userDTO{
			ID:         u.ID.String(),
			TelegramID: u.TelegramID,
			Username:   u.Username,
			FirstName:  u.FirstName,
			LastName:   u.LastName,
		},
		Settings: settingsDTO{
			NotificationsEnabled: true,
			PreferredLanguage:    "en",
		},
	})
}
```

- [ ] **Step 2: Verify build**

```bash
go build ./... 2>&1 | tail -3
```
Expected: exits 0.

- [ ] **Step 3: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/bots/rolltonchatbot/handlers/http/me_handler.go
git commit -m "feat(bot): /api/v1/me handler — returns user + default settings"
```

---

## Task 6: Wire `/api/v1/me` + CORS into rolltonchatbot

**Files:**
- Modify: `bot/internal/bots/rolltonchatbot/wire.go`

- [ ] **Step 1: Read current wire.go**

Confirm current shape — should match what's there from the user-registration commit.

- [ ] **Step 2: Apply edits**

In `bot/internal/bots/rolltonchatbot/wire.go`, change the imports block to add `"time"` if not already there.

Replace the `NewApp` function. Find this block:

```go
	mux := httpstd.NewServeMux()
	mux.Handle("/healthz", httph.NewHealthzHandler())

	return &App{deps: deps, bot: b, mux: mux}, nil
```

Replace with:

```go
	mux := httpstd.NewServeMux()
	mux.Handle("/healthz", httph.NewHealthzHandler())
	mux.Handle("/api/v1/me", middleware.TmaAuth(
		deps.Cfg.TelegramToken,
		deps.UserSvc,
		deps.Log,
		24*time.Hour,
	)(httph.NewMeHandler()))

	return &App{deps: deps, bot: b, mux: mux}, nil
```

Now find this block (in `(*App).Run`):

```go
	srv := &httpstd.Server{
		Addr:              fmt.Sprintf(":%d", a.deps.Cfg.HTTP.Port),
		Handler:           middleware.HTTP(a.deps.Log)(a.mux),
		ReadHeaderTimeout: 5 * time.Second,
	}
```

Replace `Handler` with:

```go
		Handler: middleware.CORS(a.deps.Cfg.HTTP.AllowedOrigins)(
			middleware.HTTP(a.deps.Log)(a.mux),
		),
```

- [ ] **Step 3: Build**

```bash
go build ./... 2>&1 | tail -3
```
Expected: exits 0.

- [ ] **Step 4: Run all tests as regression check**

```bash
go test -race -short ./... 2>&1 | tail -10
```
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add bot/internal/bots/rolltonchatbot/wire.go
git commit -m "feat(bot): wire /api/v1/me with TmaAuth + CORS into rolltonchatbot"
```

---

## Task 7: Frontend `useMe()` hook

**Files:**
- Create: `web/src/hooks/useMe.ts`

- [ ] **Step 1: Make the hooks dir**

```bash
mkdir -p /Users/konstantinkotau/Desktop/projects.com/rollton/web/src/hooks
```

- [ ] **Step 2: Write `useMe.ts`**

```ts
import { useQuery } from '@tanstack/react-query'
import { me } from '@/api/user'

export function useMe() {
  return useQuery({
    queryKey: ['me'],
    queryFn: me,
    staleTime: 5 * 60_000,
    retry: 1,
  })
}
```

- [ ] **Step 3: Typecheck**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run typecheck 2>&1 | tail -3
```
Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/hooks/useMe.ts
git commit -m "feat(web): useMe() hook wrapping TanStack Query"
```

---

## Task 8: `AuthBoundary` component + test (TDD)

**Files:**
- Create: `web/src/components/AuthBoundary.tsx`
- Create: `web/src/test/AuthBoundary.test.tsx`
- Modify: `web/src/test/setup.tsx` (add default MSW handler for `/api/v1/me`)

- [ ] **Step 1: Add MSW default handler for `/api/v1/me` in setup.tsx**

Open `web/src/test/setup.tsx`. Add at the top of the file (alongside the existing imports):

```ts
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
```

Then add this block before the existing `vi.stubGlobal`:

```ts
// Default MSW server with a permissive /api/v1/me handler. Tests that need
// other behaviors override per-test via server.use(...).
const defaultMeHandler = http.get('*/api/v1/me', () =>
  HttpResponse.json({
    success: true,
    data: {
      user: {
        id: 'mock-id',
        telegram_id: 42,
        first_name: 'Test',
        username: 'testuser',
      },
      settings: { notifications_enabled: true, preferred_language: 'en' },
    },
  }),
)

export const server = setupServer(defaultMeHandler)
server.listen({ onUnhandledRequest: 'bypass' })
```

- [ ] **Step 2: Write the failing test**

Create `web/src/test/AuthBoundary.test.tsx`:

```tsx
import { describe, it, expect, afterEach } from 'vitest'
import { http, HttpResponse, delay } from 'msw'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { renderWithProviders, server } from '@/test/setup'
import { AuthBoundary } from '@/components/AuthBoundary'

afterEach(() => server.resetHandlers())

describe('AuthBoundary', () => {
  it('renders children on success', async () => {
    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )
    expect(await screen.findByText('Protected content')).toBeInTheDocument()
  })

  it('shows "Open from Telegram" on 401', async () => {
    server.use(
      http.get('*/api/v1/me', () =>
        HttpResponse.json(
          { success: false, error: { code: 'UNAUTHORIZED', message: 'invalid init data' } },
          { status: 401 },
        ),
      ),
    )
    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )
    expect(await screen.findByText(/Open from Telegram/i)).toBeInTheDocument()
    expect(screen.queryByText('Protected content')).not.toBeInTheDocument()
  })

  it('shows retry on 500 and re-fetches when clicked', async () => {
    let calls = 0
    server.use(
      http.get('*/api/v1/me', () => {
        calls++
        if (calls === 1) {
          return HttpResponse.json(
            { success: false, error: { code: 'INTERNAL_ERROR', message: 'db down' } },
            { status: 500 },
          )
        }
        return HttpResponse.json({
          success: true,
          data: {
            user: { id: 'mock-id', telegram_id: 42, first_name: 'Test' },
            settings: { notifications_enabled: true, preferred_language: 'en' },
          },
        })
      }),
    )

    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )

    const retryBtn = await screen.findByRole('button', { name: /retry/i })
    await userEvent.click(retryBtn)
    expect(await screen.findByText('Protected content')).toBeInTheDocument()
  })

  it('shows a loading state while the request is in flight', async () => {
    server.use(
      http.get('*/api/v1/me', async () => {
        await delay(50)
        return HttpResponse.json({
          success: true,
          data: {
            user: { id: 'mock-id', telegram_id: 42, first_name: 'Test' },
            settings: { notifications_enabled: true, preferred_language: 'en' },
          },
        })
      }),
    )

    renderWithProviders(
      <AuthBoundary>
        <div>Protected content</div>
      </AuthBoundary>,
    )
    // children NOT visible during loading
    expect(screen.queryByText('Protected content')).not.toBeInTheDocument()
    // resolves eventually
    expect(await screen.findByText('Protected content')).toBeInTheDocument()
  })
})
```

- [ ] **Step 3: Run, expect compile/import fail**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run test 2>&1 | tail -10
```
Expected: cannot import `@/components/AuthBoundary`.

- [ ] **Step 4: Write `AuthBoundary.tsx`**

Create `web/src/components/AuthBoundary.tsx`:

```tsx
import type { ReactNode } from 'react'
import { Spinner } from '@telegram-apps/telegram-ui'
import { useMe } from '@/hooks/useMe'
import { ErrorState } from '@/components/ErrorState'

export function AuthBoundary({ children }: { children: ReactNode }) {
  const { data, error, isLoading, refetch } = useMe()
  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />
  return <>{children}</>
}
```

- [ ] **Step 5: Run tests**

```bash
npm run test 2>&1 | tail -10
```
Expected: 4 AuthBoundary tests PASS + existing HomePage test PASS.

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/components/AuthBoundary.tsx web/src/test/AuthBoundary.test.tsx web/src/test/setup.tsx
git commit -m "feat(web): AuthBoundary component + MSW default handler for /api/v1/me"
```

---

## Task 9: Layout — wrap routes in `AuthBoundary` + `WelcomeHeader`

**Files:**
- Modify: `web/src/components/Layout.tsx`

- [ ] **Step 1: Replace `Layout.tsx`**

```tsx
import { Outlet } from 'react-router-dom'
import { AppRoot } from '@telegram-apps/telegram-ui'
import { useIsTelegramEnv } from '@/lib/telegram'
import { useMe } from '@/hooks/useMe'
import { AuthBoundary } from '@/components/AuthBoundary'
import { BottomNav } from '@/components/BottomNav'
import { OutsideTelegramNotice } from '@/components/OutsideTelegramNotice'

export function Layout() {
  const inTelegram = useIsTelegramEnv()
  if (!inTelegram) {
    return (
      <AppRoot>
        <OutsideTelegramNotice />
      </AppRoot>
    )
  }
  return (
    <AppRoot>
      <AuthBoundary>
        <div className="flex h-full flex-col">
          <WelcomeHeader />
          <main className="flex-1 overflow-auto">
            <Outlet />
          </main>
          <BottomNav />
        </div>
      </AuthBoundary>
    </AppRoot>
  )
}

function WelcomeHeader() {
  const { data } = useMe()
  if (!data) return null
  return (
    <header className="border-b px-4 py-2 text-sm">
      Hi, {data.user.first_name}
    </header>
  )
}
```

- [ ] **Step 2: Typecheck + build**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run typecheck && npm run build 2>&1 | tail -5
```
Expected: typecheck OK; build produces `dist/`.

- [ ] **Step 3: Run all tests (regression)**

```bash
npm run test 2>&1 | tail -10
```
Expected: AuthBoundary tests still PASS; HomePage test still PASS (default MSW handler covers `/api/v1/me`, so Layout's AuthBoundary resolves before HomePage's `findByText('Sherlock')` runs).

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/components/Layout.tsx
git commit -m "feat(web): wrap routes in AuthBoundary + render WelcomeHeader"
```

---

## Task 10: Verification (no Docker)

- [ ] **Step 1: Backend — full unit suite**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/bot
make test 2>&1 | tail -10
```
Expected: all PASS — new TMA auth tests, new CORS tests, new config tests, plus all previous tests.

- [ ] **Step 2: Backend — lint**

```bash
make lint 2>&1 | tail -3
```
Expected: clean.

- [ ] **Step 3: Backend — build**

```bash
make build 2>&1 | tail -5
```
Expected: `bin/admin`, `bin/rolltonchatbot` updated.

- [ ] **Step 4: Frontend — test + typecheck + lint + build**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run test && npm run typecheck && npm run lint && npm run build 2>&1 | tail -5
```
Expected: all four pass.

- [ ] **Step 5: Tree clean**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git status
```
Expected: `nothing to commit, working tree clean`.

- [ ] **Step 6: Push**

```bash
git push origin main
```

---

## Live verification (deferred — requires Docker + a Cloudflare tunnel)

Run after Docker is up and the operator has a way to expose the API over HTTPS:

1. `docker compose up -d postgres`, `make migrate-up` (apply users table migration).
2. Configure `bot/.env`: `CORS_ALLOWED_ORIGINS=https://app.rollton.com,http://localhost:5173`.
3. Run rolltonchatbot: `bin/rolltonchatbot`.
4. In another terminal, hand-sign a valid initData (or use the test helper) and curl:
   ```bash
   curl -H "Authorization: tma <signed>" http://localhost:8080/api/v1/me | python3 -m json.tool
   ```
   Expected: 200 with `{success:true, data:{user:{...}, settings:{notifications_enabled:true, preferred_language:"en"}}}`. A `users` row exists.
5. Expose `api.rollton.com` via tunnel + DNS. Expose web via Cloudflare Pages.
6. Open `@rolltonchatbot` in Telegram → tap menu button → mini-app loads → "Hi, <first_name>" appears.

---

## Out-of-scope (intentional, deferred to later plans)

- Settings persistence (separate `user_settings` table + `PATCH /api/v1/settings`).
- Cookie / JWT session caching.
- Rate limiting on `/api/v1/*`.
- Refreshing initData mid-session (currently: 401 → reopen via menu button).
- Telegram-side `/start` handler changes (untouched).
- `admin` bot HTTP API.

## Open items resolved at execution time

- `init-data-golang` exact API shape — `Validate`, `Parse`, `Sign` are the assumed names. Minor version drift may rename; adjust the imports / test helpers accordingly.
- `CORS_ALLOWED_ORIGINS` production value (`https://app.rollton.com`) is set in deployment, not in code.
- Whether to also gate `/healthz` behind CORS (currently CORS wraps everything; `/healthz` itself is unauthenticated — fine).
