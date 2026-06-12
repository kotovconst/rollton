# Rolltonchatbot Mini App (`web/`) — structure & design

**Date:** 2026-06-12
**Status:** Approved (design phase)
**Scope:** Scaffold a Telegram Mini App at `web/` in the `rollton` monorepo. Only the `rolltonchatbot` (main bot, the "launcher") has a frontend. Stub-only: project structure, plumbing, one smoke test. No real product features yet.

## 1. Context

Rollton is a Character.AI-style product hosted entirely on Telegram. Architecture:

- **`rolltonchatbot`** — the launcher. Users browse characters, manage settings, and manage subscription via this bot's Telegram Mini App. Each character "Open chat" action deep-links to a per-character Telegram bot.
- **Per-character bots** — one Telegram bot per character. No frontend; chat happens natively in Telegram.
- **`admin`** — internal/admin bot. No frontend.

This spec covers only `rolltonchatbot`'s mini app.

## 2. Stack decisions

| Concern | Choice | Why |
|---|---|---|
| Build tool | Vite | Industry standard for React in 2026; sub-second HMR |
| Language | TypeScript | Mandatory at this complexity |
| UI framework | React 19 | Stated requirement |
| State (client) | Zustand | Stated requirement |
| State (server) | TanStack Query | Pairs with Zustand canonically; Zustand isn't built for server cache |
| Telegram SDK | `@telegram-apps/sdk-react` | Official, replaces deprecated `@twa-dev/sdk` |
| Component library | `@telegram-apps/telegram-ui` | Official React UI kit; native Telegram look — matches product positioning |
| CSS utilities | Tailwind CSS | For layout/spacing telegram-ui doesn't cover |
| Routing | React Router 7 (data mode) | Familiar, well-supported |
| Tests | Vitest + React Testing Library + MSW | Standard Vite test stack |
| Lint/format | ESLint v9 (flat config) + Prettier | Conventional |
| Mocking | `vi.mock` + MSW | Unit + network layers |
| Auth wire format | `Authorization: tma <initDataRaw>` | Per Telegram Mini App convention |
| Deploy target | Unspecified (stub) | TBD: Cloudflare Pages / GH Pages / embedded in Go bot |

## 3. Directory layout

```
rollton/
├── bot/                                # already scaffolded
├── infra/                              # placeholder
└── web/
    ├── package.json
    ├── tsconfig.json
    ├── tsconfig.node.json              # for vite.config.ts toolchain
    ├── vite.config.ts                  # vite + plugins (react, tailwind, vitest)
    ├── tailwind.config.ts
    ├── postcss.config.js
    ├── index.html
    ├── .env.example                    # documents VITE_* env vars
    ├── .env.development                # local dev defaults
    ├── eslint.config.js                # ESLint v9 flat config
    ├── .prettierrc
    ├── .gitignore
    ├── README.md
    ├── public/
    │   └── favicon.svg
    └── src/
        ├── main.tsx                    # entrypoint
        ├── App.tsx                     # providers + router composition root
        ├── lib/
        │   ├── telegram.ts             # SDK init + hooks + deep-link helper
        │   └── queryClient.ts          # TanStack Query client singleton
        ├── api/
        │   ├── client.ts               # apiFetch — auth + envelope unwrap
        │   ├── characters.ts
        │   ├── user.ts
        │   └── subscription.ts
        ├── stores/
        │   ├── userStore.ts            # zustand: telegram user + settings
        │   └── uiStore.ts              # zustand: UI flags
        ├── pages/
        │   ├── HomePage.tsx            # character grid
        │   ├── CharacterPage.tsx       # detail + Open-chat deep-link
        │   ├── SettingsPage.tsx
        │   └── SubscriptionPage.tsx
        ├── components/
        │   ├── Layout.tsx              # telegram-ui AppRoot shell + Outlet
        │   ├── BottomNav.tsx
        │   ├── CharacterCard.tsx
        │   ├── ErrorBoundary.tsx
        │   ├── ErrorState.tsx          # code-aware error UI
        │   └── OutsideTelegramNotice.tsx
        ├── types/
        │   └── api.ts                  # wire-format types
        ├── styles/
        │   └── index.css               # tailwind + telegram-ui imports
        └── test/
            ├── setup.ts                # vitest setup, SDK mock, helpers
            └── HomePage.test.tsx       # smoke test
```

## 4. Component responsibilities

### 4.1 Entry + providers

**`main.tsx`** — runs `initSDK()` once, mounts `<App />` into `#root`, imports `./styles/index.css`.

**`App.tsx`** — provider order:
```
<SDKProvider>
  <AppRoot>                  ← telegram-ui theme + safe-area
    <QueryClientProvider>
      <ErrorBoundary>
        <RouterProvider router={router} />
      </ErrorBoundary>
    </QueryClientProvider>
  </AppRoot>
</SDKProvider>
```
Router uses `createBrowserRouter([...])` with one parent `<Layout>` route wrapping `/`, `/characters/:id`, `/settings`, `/subscription`.

### 4.2 Plumbing (`lib/`)

**`telegram.ts`** exports:
- `initSDK()` — idempotent; `init()` + `miniApp.ready()` + `viewport.expand()`.
- `useTelegramUser()` — typed Telegram user from `initData`.
- `useInitDataRaw()` — raw `initData` string for backend HMAC validation.
- `useIsTelegramEnv()` — boolean, false in plain browser.
- `openCharacterBot(username, payload)` — wraps `utils.openTelegramLink`.

**`queryClient.ts`** — `new QueryClient({ defaultOptions: { queries: { staleTime: 30_000, retry: 1 } } })`.

### 4.3 API layer (`api/`)

**`client.ts`** — single `apiFetch<T>(path, init?): Promise<T>`:
- Base URL from `import.meta.env.VITE_API_BASE_URL`.
- `Authorization: tma ${initDataRaw}` (from SDK runtime).
- JSON content-type on non-GET.
- Unwraps backend envelope `{success, data, error}` → `data` on success.
- Throws typed `ApiError` on failure (see Section 6).

**`characters.ts`** — `listCharacters()`, `getCharacter(id)`.
**`user.ts`** — `me()`, `updateSettings(input)`.
**`subscription.ts`** — `getSubscription()`.

### 4.4 Stores (`stores/`)

**`userStore.ts`** — Zustand:
- `tgUser: TelegramUser | null` — populated once at boot.
- `settings: UserSettings | null` — hydrated from `me()`.
- actions: `setTgUser`, `setSettings`.

**`uiStore.ts`** — Zustand placeholder for modals/toggles.

**Why both Zustand and TanStack Query:** Zustand owns client state (UI flags, ambient user). TanStack Query owns server state (characters, subscription). Pages consume both as appropriate.

### 4.5 Pages (`pages/`)

Each page: route component, uses `useQuery` for server data, renders telegram-ui components + Tailwind. Stub content is fine — the goal is to prove plumbing works, not to implement features.

- **HomePage** — `useQuery(['characters'], listCharacters)`, maps to `<CharacterCard>` grid.
- **CharacterPage** — `useQuery(['character', id], () => getCharacter(id))`, "Open chat" button calls `openCharacterBot(...)`.
- **SettingsPage** — reads `userStore.settings`, `useMutation(updateSettings)` on change.
- **SubscriptionPage** — reads `useQuery(['subscription'], getSubscription)`. Read-only.

### 4.6 Components (`components/`)

- **Layout** — `<AppRoot>` shell, optional header, `<Outlet />`, `<BottomNav />`. Short-circuits to `<OutsideTelegramNotice>` when `useIsTelegramEnv()` is false.
- **BottomNav** — three `<NavLink>`s: Home / Settings / Subscription.
- **CharacterCard** — dumb; avatar + name + blurb from props.
- **ErrorBoundary** — class component; catches render errors.
- **ErrorState** — code-aware error UI consumed by pages.
- **OutsideTelegramNotice** — guidance text for dev'ing in a browser.

### 4.7 Types (`types/api.ts`)
Hand-written wire types: `Character`, `User`, `UserSettings`, `Subscription`, `ApiEnvelope<T>`, `ApiError`. Replace with generated types if/when backend ships OpenAPI.

### 4.8 Styles
`src/styles/index.css`:
```css
@tailwind base;
@tailwind components;
@tailwind utilities;
@import '@telegram-apps/telegram-ui/dist/styles.css';
```
Order matters: Tailwind utilities override telegram-ui layout where needed.

### 4.9 Tests (`src/test/`)

- **`setup.ts`** — `@testing-library/jest-dom` import; `vi.mock('@telegram-apps/sdk-react', …)` returning a stable fake user + `initDataRaw: 'mock'`; exports a `renderWithProviders` helper that wraps in `MemoryRouter` + `QueryClientProvider` + mocked SDK.
- **`HomePage.test.tsx`** — renders `<HomePage />` via `renderWithProviders`; MSW handler returns mocked characters; asserts loading then card render.

## 5. Data flow

### 5.1 Boot
```
index.html → main.tsx
  ├── import './styles/index.css'
  ├── initSDK()
  └── createRoot.render(<App />)
        └── SDKProvider populates initData from window.Telegram.WebApp
              └── RouterProvider renders HomePage at "/"
```

### 5.2 Request
```
Component → useQuery → api/<resource>.ts → apiFetch
   ├── VITE_API_BASE_URL + path
   ├── Authorization: tma <initDataRaw>
   └── fetch → backend
         ├── middleware validates HMAC(initData, bot_token)
         ├── handler returns {success:true, data:…}
         ↓
   apiFetch unwraps .data
   ↓
   useQuery hydrates cache → component re-renders
```

### 5.3 Cross-bot deep-link
```
"Open chat" on CharacterPage
  → openCharacterBot('sherlock_rollton_bot', `ref_${tgUser.id}`)
  → utils.openTelegramLink('https://t.me/sherlock_rollton_bot?start=ref_<id>')
  → Telegram closes mini app, opens character bot's chat
  → character bot reads /start payload, records arrival source
```

`Character.botUsername` is therefore part of the API contract.

## 6. Error handling

**Three failure surfaces:**

1. **Render errors** — `<ErrorBoundary>` catches, shows telegram-ui placeholder with "Reload" button. No external reporting in stub.
2. **API errors** — `apiFetch` always throws typed `ApiError { status, code, message }`. Pages render `<ErrorState error={error} />` driven by `code`:
   - `UNAUTHORIZED` → "Open from inside Telegram." (no retry)
   - `NETWORK` / `TIMEOUT` → "Connection problem." (retry button)
   - other → generic "Something went wrong."
3. **Outside Telegram** — `useIsTelegramEnv() === false` → `<Layout>` renders `<OutsideTelegramNotice>` instead of routes. Allows `npm run dev` to open in plain browser without crashing.

**Mutations** use `useMutation` `onError` to show telegram-ui `<Snackbar>`. No optimistic updates in the stub.

**Cancellation** — Pass `AbortSignal` from `useQuery` through `apiFetch` to `fetch`.

## 7. Testing

Stub scope: prove the toolchain runs.

| File | Covers | Style |
|---|---|---|
| `src/test/setup.ts` | jsdom, jest-dom, mocked SDK, `renderWithProviders` | Loaded via vitest config |
| `src/test/HomePage.test.tsx` | HomePage mounts, loading → cards | RTL + MSW |

**Out of scope for stub:** route tests, mutation tests, a11y, visual regression. Add with real features.

**Scripts:**
- `npm run test` → `vitest run` (CI-mode).
- `npm run test:watch` → `vitest`.

## 8. Build, dev, deploy

**Dev:**
```bash
npm install
npm run dev          # Vite on http://localhost:5173, HMR
```
For Telegram-flavored dev: point a tunnel (cloudflared / ngrok) at `localhost:5173` and set @BotFather's web_app_url to the tunnel URL.

**Build:**
```bash
npm run build        # → web/dist/
npm run preview      # smoke-test prod bundle locally
```

**Lint / format / typecheck:**
```bash
npm run lint         # eslint
npm run format       # prettier --write
npm run typecheck    # tsc --noEmit
```

**Env vars** (must be `VITE_`-prefixed to leak into the bundle):
- `VITE_API_BASE_URL` — backend base URL.
- `VITE_BOT_USERNAME` — main bot username for telegram link helpers.

`.env.development` ships defaults: `http://localhost:8080`, `rolltonchatbot`.
`.env.example` documents both. `.env.local` is gitignored for per-machine overrides.

**Deploy:** out of scope. Likely options (decide at deploy time):
- Cloudflare Pages (recommended: free, fast, easy)
- GitHub Pages
- Embed `web/dist/` in the Go bot's HTTP server via `embed.FS`

## 9. Backend contract (cross-component, not in scope to implement here)

The mini app expects these endpoints on `rolltonchatbot`'s HTTP server. Implementing them is post-scaffold work; listed here so both sides stay aligned.

| Method | Path | Body / Query | Response `data` |
|---|---|---|---|
| GET | `/api/v1/me` | — | `{ user: User, settings: UserSettings }` |
| GET | `/api/v1/characters` | — | `Character[]` |
| GET | `/api/v1/characters/:id` | — | `Character` |
| PATCH | `/api/v1/settings` | `Partial<UserSettings>` | `UserSettings` |
| GET | `/api/v1/subscription` | — | `Subscription` |

All require `Authorization: tma <initDataRaw>`. The Go middleware (future work) validates HMAC-SHA256 using `TELEGRAM_TOKEN` via `github.com/telegram-mini-apps/init-data-golang` (or equivalent).

## 10. State ownership

| Concern | Lives in | Why |
|---|---|---|
| Telegram user identity | `userStore.tgUser` | Single source for the tree |
| User settings | `userStore.settings`, hydrated by `useQuery('me')` | Mutations write to both server + store |
| Character list / detail | TanStack Query cache | Server-of-truth, cacheable |
| Active character on a page | URL param `:id` | Cheap, back-button friendly |
| Modals, UI toggles | `uiStore` | Transient client state |
| `initDataRaw` | Pulled from SDK at request time | Always fresh |

## 11. Explicit non-goals (stub scope boundary)

- No real product features (no character data, no chat, no subscription checkout).
- No backend endpoints (Go side untouched in this scaffold).
- No CI workflow.
- No analytics, telemetry, or Sentry.
- No deployment target chosen / configured.
- No multi-locale / i18n yet (English-only).
- No PWA / service worker.
- No styling system beyond Tailwind + telegram-ui defaults.

## 12. Open items resolved at execution time

- Node version: pin to current LTS at scaffold time (Node 22 LTS as of mid-2026).
- Package manager: default to `npm`. Switch to `pnpm` if monorepo manages multiple JS packages later.
- ESLint v9 flat config syntax may shift between minor versions — pin at scaffold.
- Tailwind v4 vs v3: pick v4 (released, stable as of 2026) unless tooling friction appears.
