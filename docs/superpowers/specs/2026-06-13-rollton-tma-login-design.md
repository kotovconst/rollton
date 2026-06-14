# Telegram Mini App login (TMA initData → /api/v1/me) — design

**Date:** 2026-06-13
**Status:** Approved (design phase)
**Scope:** Close the auth loop between the rolltonchatbot mini-app (SPA) and the bot's HTTP API. After this spec: a user opens the mini-app from inside Telegram, no interaction required, and the SPA shows a personalized header backed by a verified backend identity.

## 1. Context

Two pieces already exist:

- **Backend Telegram path:** `EnsureUserRegistered` middleware auto-registers anyone who sends a Telegram update. Lives in `bot/internal/middleware/auth.go`.
- **Frontend `apiFetch`:** Already injects `Authorization: tma <initDataRaw>` on every request. Lives in `web/src/api/client.ts`.

What's missing is the **HTTP-side validation and identity endpoint**:

- No middleware verifies the `Authorization: tma ...` HMAC.
- `/api/v1/me` doesn't exist; the mini-app's `useMe()` returns 404.
- The mini-app's Layout has no auth gate, so child pages can render before any identity is known.

This spec adds those three pieces.

## 2. Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Auth wire format | `Authorization: tma <initDataRaw>` | Telegram Mini App convention; already wired in `apiFetch` |
| initData validation library | `github.com/telegram-mini-apps/init-data-golang` | Official-ish; HMAC + `auth_date` freshness in one call |
| initData max age | 24 hours | Sensible default; matches Telegram's recommendation |
| Identity source | `parsed.User` from validated initData | Trustworthy after HMAC; carries `IsPremium` that tgbotapi/v5 omits |
| Cross-bot user table | Single `users` table, shared with Telegram-update path | Already exists; same `UserService.EnsureRegistered` reused |
| Session caching | None — re-validate HMAC on every request | Cheap (microseconds, no network); revisit only if profiling demands |
| Response shape for `/api/v1/me` | `{user, settings}` with hardcoded default settings | Forward-compatible with the SPA; defers settings persistence to a separate spec |
| Settings persistence | Out of scope | Hardcoded defaults: `notifications_enabled=true, preferred_language="en"` |
| Frontend state model | TanStack Query owns server state | `useMe()` hook = `useQuery(['me'], me)`; Zustand stays for client state only (modals, toggles) |
| Failure surface to client | Always 401 `UNAUTHORIZED` for any HMAC/parse/freshness issue | Security best practice — don't leak why validation failed |
| CORS | Permissive for configured origins | SPA at `app.rollton.com` calls API at `api.rollton.com` — cross-origin |
| Out-of-scope binaries | Only `rolltonchatbot` exposes `/api/v1/*` | `admin` bot has no mini-app |

## 3. Backend (Go) components

### 3.1 `internal/middleware/tma_auth.go`
HTTP middleware. Gates `/api/v1/*`.

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
				parsed.User.ID, parsed.User.Username,
				parsed.User.FirstName, parsed.User.LastName,
				parsed.User.LanguageCode, parsed.User.IsPremium,
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

Reuses the existing `UserService` interface and `WithUser`/`UserFromContext` helpers from `auth.go`. No new ports.

### 3.2 `internal/middleware/cors.go`
Standard CORS middleware. Reads allowed origins from config (slice). Handles OPTIONS preflight, echoes the `Origin` header when matched.

Methods: `GET POST PATCH OPTIONS`. Headers: `Authorization Content-Type`. Credentials: not allowed (we use header-based auth, not cookies).

### 3.3 `internal/bots/rolltonchatbot/handlers/http/me_handler.go`
```go
type MeResponse struct {
	User     UserDTO     `json:"user"`
	Settings SettingsDTO `json:"settings"`
}

type UserDTO struct {
	ID         string `json:"id"`
	TelegramID int64  `json:"telegram_id"`
	Username   string `json:"username,omitempty"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name,omitempty"`
}

type SettingsDTO struct {
	NotificationsEnabled bool   `json:"notifications_enabled"`
	PreferredLanguage    string `json:"preferred_language"`
}

func (h *MeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	u, ok := middleware.UserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.ErrCodeUnauthorized, "no user in context")
		return
	}
	httpx.WriteSuccess(w, MeResponse{
		User: UserDTO{
			ID: u.ID.String(), TelegramID: u.TelegramID,
			Username: u.Username, FirstName: u.FirstName, LastName: u.LastName,
		},
		Settings: SettingsDTO{NotificationsEnabled: true, PreferredLanguage: "en"},
	})
}
```

Settings are hardcoded — replaced when the settings spec lands.

### 3.4 Wire changes in `internal/bots/rolltonchatbot/wire.go`
```go
mux := httpstd.NewServeMux()
mux.Handle("/healthz", httph.NewHealthzHandler())
mux.Handle("/api/v1/me", middleware.TmaAuth(
	deps.Cfg.TelegramToken,
	deps.UserSvc,
	deps.Log,
	24*time.Hour,
)(httph.NewMeHandler()))

handler := middleware.CORS(deps.Cfg.HTTP.AllowedOrigins)(
	middleware.HTTP(deps.Log)(mux),
)
```

- `/healthz` is **unauthenticated**.
- `/api/v1/me` is gated by `TmaAuth`.
- `CORS` wraps the whole mux so preflight works for any future `/api/v1/*` route.
- `HTTP` (logging) middleware wraps everything inside CORS so OPTIONS shows up in logs too.

The `admin` bot's wire is untouched — admin has no mini-app.

### 3.5 Config additions (`internal/config/config.go`)
```go
type HTTPConfig struct {
	Port           int
	AllowedOrigins []string
}
```

New env var: `CORS_ALLOWED_ORIGINS` — comma-separated. Defaults to empty (CORS denies cross-origin requests by default). Production set to `https://app.rollton.com`. Dev `.env` set to `*` (or to the local tunnel URL).

### 3.6 Dependency additions
- `go get github.com/telegram-mini-apps/init-data-golang@latest` — new lib.

## 4. Frontend components

### 4.1 `web/src/hooks/useMe.ts` (new)
```ts
import { useQuery } from '@tanstack/react-query'
import { me } from '@/api/user'

export function useMe() {
  return useQuery({ queryKey: ['me'], queryFn: me, staleTime: 5 * 60_000 })
}
```

### 4.2 `web/src/components/AuthBoundary.tsx` (new)
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

### 4.3 `web/src/components/Layout.tsx` (modified)
Adds `AuthBoundary` around in-Telegram routes and a tiny welcome header so "login worked" is visually obvious.

```tsx
export function Layout() {
  const inTelegram = useIsTelegramEnv()
  if (!inTelegram) return <AppRoot><OutsideTelegramNotice /></AppRoot>
  return (
    <AppRoot>
      <AuthBoundary>
        <div className="flex h-full flex-col">
          <WelcomeHeader />
          <main className="flex-1 overflow-auto"><Outlet /></main>
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

`WelcomeHeader` reads from the same query cache (no second fetch).

### 4.4 No Zustand mutation in this slice
Server state owned by TanStack Query. `userStore.tgUser` / `userStore.settings` remain in the codebase as scaffolding for *future* needs (e.g. optimistic update buffers) but aren't populated here.

### 4.5 `web/src/types/api.ts` — adjust `me()` response type
Current type: `me(): Promise<{ user: User; settings: UserSettings }>` — already matches what the backend will return. No change needed.

## 5. Data flow

```
User taps menu button in @rolltonchatbot
   │
   ▼
Telegram WebView opens https://app.rollton.com
   ├── injects window.Telegram.WebApp.initData
   └── injects window.Telegram.WebApp.initDataUnsafe.user (untrusted)
   │
   ▼
main.tsx → initSDK() → createRoot.render(<App />)
   │
   ▼
Layout (isTelegramEnv=true) → <AuthBoundary>
   │
   ▼
useMe() → apiFetch('/api/v1/me')
   └── header: Authorization: tma <initData>
   │
   ▼
GET https://api.rollton.com/api/v1/me
   │
   ▼
CORS middleware (preflight already done separately by browser)
   │
   ▼
TmaAuth middleware
   ├── parse "tma " prefix → raw
   ├── initdata.Validate(raw, TELEGRAM_TOKEN, 24h) → HMAC + auth_date check
   ├── initdata.Parse(raw) → {User: {...}}
   ├── domain.NewUser(...)
   ├── userSvc.EnsureRegistered(ctx, user) — same service as Telegram path
   └── ctx = WithUser(r.Context(), &u)
   │
   ▼
MeHandler reads user from ctx
   └── 200 {success:true, data:{user, settings:{defaults}}}
   │
   ▼
AuthBoundary renders <Outlet />
WelcomeHeader renders "Hi, <first_name>"
```

## 6. Error handling

| Stage | Failure | HTTP status | UI behaviour |
|---|---|---|---|
| Not in Telegram | `isTelegramEnv()` false | (no request fired) | `<OutsideTelegramNotice>` |
| Missing `Authorization` header | Defensive | 401 `UNAUTHORIZED` | `ErrorState` "Open from Telegram" — no retry |
| Bad HMAC | Forged or wrong-bot initData | 401 `UNAUTHORIZED` | Same |
| Stale `auth_date` (>24h) | Resumed long-idle session | 401 `UNAUTHORIZED` | Same — user reopens via menu button for fresh initData |
| `EnsureRegistered` fails | DB outage | 500 `INTERNAL_ERROR` | `ErrorState` "Connection problem" with retry |
| Network / timeout | Tunnel / DNS / CDN | n/a (fetch throws) | `ErrorState` same |

`ErrorState` already supports these codes — no changes needed.

**Security note:** All HMAC/parse/freshness failures collapse to the same `401 UNAUTHORIZED` response. Internal logs record the specific reason; the wire response does not (don't help attackers triangulate).

## 7. Testing

### Backend

**`bot/internal/middleware/test/tma_auth_test.go`**

| Test | Assertion |
|---|---|
| Valid initData → 200; downstream sees user in ctx | Status 200, fakeUserSvc.called=true |
| Missing header → 401 UNAUTHORIZED; service NOT called | Status 401; body code=UNAUTHORIZED; fakeUserSvc.called=false |
| Wrong prefix → 401; service NOT called | Same |
| Bad HMAC → 401; service NOT called | Same |
| Stale auth_date → 401; service NOT called | Same |
| Valid initData + service error → 500 INTERNAL_ERROR | Status 500; code=INTERNAL_ERROR; service WAS called |

Test helper: `signInitData(t, botToken, user, authDate) string` builds a valid signed initData query string using the init-data-golang lib's `Sign` function (lib provides it for testing).

**`bot/internal/middleware/test/cors_test.go`**

| Test | Assertion |
|---|---|
| Allowed origin → response includes `Access-Control-Allow-Origin: <origin>` | header present |
| Disallowed origin → response omits ACAO header | header absent |
| OPTIONS preflight → 204 + ACAO + methods + headers | full preflight surface |

No new handler tests — `MeHandler` is glue.

### Frontend

**`web/src/test/AuthBoundary.test.tsx`**

| Test | Mock | Assertion |
|---|---|---|
| Loading state | MSW handler with `delay('infinite')` | Spinner visible; children NOT visible |
| Success | MSW returns `{success:true, data:{user:{first_name:"Alice", ...}, settings:{...}}}` | "Hi, Alice" rendered; children rendered |
| 401 | MSW returns 401 with `error.code: "UNAUTHORIZED"` | ErrorState "Open from Telegram"; no retry button |
| 500 | MSW returns 500 | ErrorState with retry button; click → refetch |

Existing `HomePage.test.tsx` gets a new default `/api/v1/me` MSW handler in `src/test/setup.tsx` so any Layout-using test gets a valid `me()` response without per-test setup.

## 8. Out-of-scope (explicit non-goals)

- **Settings persistence.** Hardcoded defaults in this slice. Separate spec for the `user_settings` table and `PATCH /api/v1/settings`.
- **Cookie / JWT sessions.** Re-validate HMAC on every request. Add sessions only if profiling shows need.
- **CSRF protection.** Custom `Authorization: tma` header can't be added cross-origin by browsers without explicit JS; no CSRF surface.
- **Rate limiting on `/api/v1/*`.** Cloudflare or Caddy can do this later.
- **Refreshing initData mid-session.** A 401 just produces an `ErrorState`; user reopens via the menu button.
- **Telegram-side `/start` handler.** Still uses the Telegram middleware path. Not touched.
- **`admin` bot.** No HTTP API, no mini-app. Untouched.
- **Public docs.** No OpenAPI / Swagger for `/api/v1/me`. Frontend types are the contract.

## 9. Open items resolved at execution time

- Exact `init-data-golang` API shape may vary by minor version. The reference imports are `initdata.Validate(raw, token, maxAge)` and `initdata.Parse(raw)`. If the library renames, adjust at execution.
- `CORS_ALLOWED_ORIGINS` value for production: `https://app.rollton.com`. For dev: set to the cloudflared tunnel URL when running through Telegram, or `*` for plain-browser dev (Layout's `isTelegramEnv` short-circuit means no API calls fire in that case anyway).
- `TELEGRAM_TOKEN` used by `TmaAuth` is the **same token** the bot uses to long-poll. The HMAC secret is the bot token itself — that's why every mini-app must be tied to exactly one bot. (Per-bot mini-apps in a multi-bot world: each bot has its own initData scope.)
