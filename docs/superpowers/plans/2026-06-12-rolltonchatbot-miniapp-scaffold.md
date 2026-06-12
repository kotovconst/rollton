# Rolltonchatbot Mini App (`web/`) — Scaffold Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold a working Telegram Mini App stub at `web/` in the `rollton` monorepo — providers, routing, pages, stores, API layer, Tailwind + telegram-ui styling, and one smoke test. No real product features.

**Architecture:** Vite + React 19 + TypeScript. Telegram glue isolated in `src/lib/telegram.ts` (uses raw `window.Telegram.WebApp` so we don't pin a specific `@telegram-apps/sdk-react` minor version). Client state in Zustand, server state in TanStack Query. React Router 7 in data-mode. Tailwind v4 (CSS-first; no `tailwind.config.ts`). One smoke test driven by MSW.

**Tech Stack:** `vite` `react` `react-dom` `typescript` `react-router-dom@7` `zustand` `@tanstack/react-query` `@telegram-apps/sdk-react` `@telegram-apps/telegram-ui` `tailwindcss@4` `@tailwindcss/vite` `vitest` `@testing-library/react` `@testing-library/jest-dom` `jsdom` `msw` `eslint@9` `prettier`.

**Reference spec:** `docs/superpowers/specs/2026-06-12-rolltonchatbot-miniapp-design.md`.

**Spec deviations to be aware of:**
- The spec listed `tailwind.config.ts` and `postcss.config.js` under Section 3. With Tailwind v4 + `@tailwindcss/vite`, neither is required. Plan omits both.
- The spec mentioned `useTelegramUser`, `useInitDataRaw`, `useIsTelegramEnv` as wrappers over `@telegram-apps/sdk-react` hooks. The plan implements them as thin facades over `window.Telegram.WebApp` (plus a tiny ambient type in `src/types/telegram.d.ts`) to insulate the codebase from SDK version churn. The SDK package is still installed because `@telegram-apps/telegram-ui` depends on it transitively in some setups, and we may use `<SDKProvider>` later.

---

## Pre-flight

Working directory for all commands: `/Users/konstantinkotau/Desktop/projects.com/rollton`. The `web/` folder does not yet exist.

| Placeholder | Value |
|---|---|
| Node version | Use whatever's installed; project pins `"engines": { "node": ">=20" }` |
| Bot username (for env defaults) | `rolltonchatbot` |
| Backend dev URL | `http://localhost:8080` |

---

## File Structure

```
web/
├── package.json                 # scripts, deps, engines
├── tsconfig.json                # references file (created by vite template)
├── tsconfig.app.json            # app code TS config
├── tsconfig.node.json           # toolchain (vite.config.ts) TS config
├── vite.config.ts               # vite + react + tailwind + vitest
├── eslint.config.js             # ESLint v9 flat config
├── .prettierrc                  # prettier config
├── .gitignore                   # node_modules, dist, .env.local
├── .env.example                 # VITE_* env var docs
├── .env.development             # local dev defaults
├── index.html                   # vite entry HTML
├── README.md                    # how to dev / build / test
├── public/
│   └── favicon.svg
└── src/
    ├── main.tsx                 # entrypoint
    ├── App.tsx                  # providers + router
    ├── lib/
    │   ├── telegram.ts          # window.Telegram.WebApp facade
    │   └── queryClient.ts       # TanStack Query client
    ├── api/
    │   ├── client.ts            # apiFetch + ApiError
    │   ├── characters.ts        # listCharacters, getCharacter
    │   ├── user.ts              # me, updateSettings
    │   └── subscription.ts      # getSubscription
    ├── stores/
    │   ├── userStore.ts         # Zustand: tgUser + settings
    │   └── uiStore.ts           # Zustand: UI flags
    ├── pages/
    │   ├── HomePage.tsx
    │   ├── CharacterPage.tsx
    │   ├── SettingsPage.tsx
    │   └── SubscriptionPage.tsx
    ├── components/
    │   ├── Layout.tsx
    │   ├── BottomNav.tsx
    │   ├── CharacterCard.tsx
    │   ├── ErrorBoundary.tsx
    │   ├── ErrorState.tsx
    │   └── OutsideTelegramNotice.tsx
    ├── types/
    │   ├── api.ts               # wire-format types
    │   └── telegram.d.ts        # ambient Window.Telegram type
    ├── styles/
    │   └── index.css            # tailwind + telegram-ui imports
    └── test/
        ├── setup.ts             # vitest setup + renderWithProviders helper
        └── HomePage.test.tsx    # smoke test
```

---

## Task 1: Bootstrap `web/` with Vite React-TS template

**Files:**
- Create directory `web/`
- Create: `web/package.json`, `web/tsconfig*.json`, `web/vite.config.ts`, `web/index.html`, `web/public/`, `web/src/`

- [ ] **Step 1: Create the project via Vite template**

Run from `rollton/`:
```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
npm create vite@latest web -- --template react-ts
```
Expected: Vite scaffolds `web/` with React + TS template files. May ask "Ok to proceed?" — answer `y`.

- [ ] **Step 2: Install template dependencies**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm install
```
Expected: `node_modules/` populated, `package-lock.json` written.

- [ ] **Step 3: Smoke-test the template (will be torn down later)**

```bash
npm run build
```
Expected: `dist/` created; build exits 0.

- [ ] **Step 4: Remove vite template's default content we'll rewrite**

```bash
rm -f src/App.tsx src/App.css src/index.css src/assets/react.svg
```

- [ ] **Step 5: Update root `.gitignore` to exclude web build artifacts**

Add these lines to `/Users/konstantinkotau/Desktop/projects.com/rollton/.gitignore` (under the existing `# Build artifacts` section):
```gitignore
# Web
web/node_modules/
web/dist/
web/.env.local
web/.env.*.local
web/coverage/
```

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add .gitignore web/
git commit -m "chore(web): bootstrap vite react-ts template"
```

---

## Task 2: Install production + dev dependencies

**Files:**
- Modify: `web/package.json`, `web/package-lock.json`

- [ ] **Step 1: Install runtime deps**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm install \
  react-router-dom@^7 \
  zustand@^5 \
  @tanstack/react-query@^5 \
  @telegram-apps/sdk-react \
  @telegram-apps/telegram-ui
```
Expected: `package.json` `dependencies` includes all five.

- [ ] **Step 2: Install build / styling deps**

```bash
npm install -D \
  tailwindcss@^4 \
  @tailwindcss/vite \
  @types/node
```

- [ ] **Step 3: Install test deps**

```bash
npm install -D \
  vitest \
  @testing-library/react \
  @testing-library/jest-dom \
  @testing-library/user-event \
  jsdom \
  msw
```

- [ ] **Step 4: Install lint / format deps**

```bash
npm install -D \
  prettier \
  eslint-config-prettier
```
(ESLint v9 + React plugins were installed by the Vite template; we only add Prettier integration.)

- [ ] **Step 5: Verify package.json state**

```bash
cat package.json | grep -A1 '"dependencies"' | head -20
```
Expected: shows the five runtime deps from Step 1.

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/package.json web/package-lock.json
git commit -m "chore(web): add runtime, build, test, and lint dependencies"
```

---

## Task 3: Configure Vite + Tailwind v4 + Vitest

**Files:**
- Modify: `web/vite.config.ts`
- Create: `web/src/styles/index.css`

- [ ] **Step 1: Replace `web/vite.config.ts`**

Overwrite `web/vite.config.ts`:
```ts
/// <reference types="vitest" />
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, 'src'),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.tsx'],
    css: true,
  },
})
```

- [ ] **Step 2: Create `web/src/styles/index.css`**

```css
@import "tailwindcss";
@import "@telegram-apps/telegram-ui/dist/styles.css";

html, body, #root {
  height: 100%;
}

body {
  margin: 0;
}
```

- [ ] **Step 3: Create the test setup directory placeholder**

```bash
mkdir -p src/test
touch src/test/setup.tsx
```
(File contents filled in Task 10. The `.tsx` extension matters because the setup file contains JSX.)

- [ ] **Step 4: Verify vite config compiles**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npx tsc --noEmit -p tsconfig.node.json
```
Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/vite.config.ts web/src/styles/index.css web/src/test/setup.ts
git commit -m "chore(web): configure vite, tailwind v4, vitest"
```

---

## Task 4: Tooling configs — tsconfig paths, ESLint flat config, Prettier

**Files:**
- Modify: `web/tsconfig.json`, `web/tsconfig.app.json` (if present)
- Modify: `web/eslint.config.js`
- Create: `web/.prettierrc`

- [ ] **Step 1: Add `@/*` path alias to `web/tsconfig.app.json`**

The Vite template creates `tsconfig.app.json` (referenced by `tsconfig.json`). Open it and ensure the `compilerOptions` block contains:
```json
"baseUrl": ".",
"paths": {
  "@/*": ["src/*"]
}
```
Add these two keys to the existing `compilerOptions` object if missing. Leave other keys alone.

- [ ] **Step 2: Write `web/.prettierrc`**

```json
{
  "semi": false,
  "singleQuote": true,
  "trailingComma": "all",
  "printWidth": 100,
  "arrowParens": "always"
}
```

- [ ] **Step 3: Extend the ESLint flat config to chain Prettier**

The Vite template wrote `web/eslint.config.js`. Open it. Find the `extends` array (or the spread of preset configs). Append `eslint-config-prettier` so Prettier disables conflicting rules. Add this import at the top:
```js
import prettier from 'eslint-config-prettier'
```
Append `prettier` to the final exports array. Concrete shape:
```js
import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import prettier from 'eslint-config-prettier'

export default tseslint.config(
  { ignores: ['dist'] },
  {
    extends: [js.configs.recommended, ...tseslint.configs.recommended, prettier],
    files: ['**/*.{ts,tsx}'],
    languageOptions: {
      ecmaVersion: 2022,
      globals: globals.browser,
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': ['warn', { allowConstantExport: true }],
    },
  },
)
```
(If the existing file differs in shape — e.g., older template — preserve its structure and just add the `prettier` import + position it last in the extends list.)

- [ ] **Step 4: Add scripts to `web/package.json`**

Open `web/package.json` and replace its `scripts` section with:
```json
"scripts": {
  "dev": "vite",
  "build": "tsc -b && vite build",
  "preview": "vite preview",
  "test": "vitest run",
  "test:watch": "vitest",
  "lint": "eslint .",
  "format": "prettier --write .",
  "format:check": "prettier --check .",
  "typecheck": "tsc -b --noEmit"
}
```
Also add at top-level:
```json
"engines": {
  "node": ">=20"
}
```

- [ ] **Step 5: Verify**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run lint 2>&1 | tail -5
```
Expected: zero errors (warnings are OK; template default).

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/tsconfig.app.json web/.prettierrc web/eslint.config.js web/package.json
git commit -m "chore(web): wire eslint+prettier, add scripts and engines"
```

---

## Task 5: Types — `api.ts` and `telegram.d.ts`

**Files:**
- Create: `web/src/types/api.ts`, `web/src/types/telegram.d.ts`

- [ ] **Step 1: Create `web/src/types/api.ts`**

```ts
// Wire-format types matching the rolltonchatbot Go backend's response envelope.

export interface ApiEnvelope<T> {
  success: boolean
  data?: T
  error?: { code: string; message: string }
}

export interface User {
  id: string
  telegram_id: number
  username?: string
  first_name?: string
  last_name?: string
}

export interface UserSettings {
  notifications_enabled: boolean
  preferred_language: string
}

export interface Character {
  id: string
  name: string
  blurb: string
  avatar_url?: string
  bot_username: string
}

export interface Subscription {
  tier: 'free' | 'pro' | 'unlimited'
  renews_at?: string
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
    this.name = 'ApiError'
  }
}
```

- [ ] **Step 2: Create `web/src/types/telegram.d.ts`**

```ts
// Minimal ambient typing for window.Telegram.WebApp.
// Tracks only the fields the stub uses.

export {}

interface TelegramUser {
  id: number
  first_name?: string
  last_name?: string
  username?: string
  language_code?: string
}

interface TelegramWebApp {
  initData: string
  initDataUnsafe: { user?: TelegramUser }
  ready(): void
  expand(): void
  openTelegramLink(url: string): void
}

declare global {
  interface Window {
    Telegram?: { WebApp?: TelegramWebApp }
  }
}
```

- [ ] **Step 3: Verify types compile**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run typecheck 2>&1 | tail -5
```
Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/types
git commit -m "feat(web): add api wire types and telegram ambient types"
```

---

## Task 6: Plumbing — `lib/telegram.ts`, `lib/queryClient.ts`, `api/*`, `stores/*`

**Files:**
- Create: `web/src/lib/telegram.ts`, `web/src/lib/queryClient.ts`
- Create: `web/src/api/client.ts`, `web/src/api/characters.ts`, `web/src/api/user.ts`, `web/src/api/subscription.ts`
- Create: `web/src/stores/userStore.ts`, `web/src/stores/uiStore.ts`

- [ ] **Step 1: `web/src/lib/telegram.ts`**

```ts
// Thin facade over window.Telegram.WebApp so the rest of the app
// never touches the global directly. Hook variants subscribe via
// React state on mount; the WebApp object itself is mutable but
// initData / user don't change after init, so a single read is enough.

import { useEffect, useState } from 'react'

export interface TelegramUser {
  id: number
  first_name?: string
  last_name?: string
  username?: string
  language_code?: string
}

export function initSDK(): void {
  const wa = window.Telegram?.WebApp
  if (!wa) return
  wa.ready()
  wa.expand()
}

export function getInitDataRaw(): string {
  return window.Telegram?.WebApp?.initData ?? ''
}

export function getTelegramUser(): TelegramUser | null {
  return window.Telegram?.WebApp?.initDataUnsafe?.user ?? null
}

export function isTelegramEnv(): boolean {
  return typeof window !== 'undefined' && !!window.Telegram?.WebApp?.initData
}

export function openCharacterBot(username: string, payload: string): void {
  const url = `https://t.me/${username}?start=${encodeURIComponent(payload)}`
  const wa = window.Telegram?.WebApp
  if (wa?.openTelegramLink) {
    wa.openTelegramLink(url)
  } else if (typeof window !== 'undefined') {
    window.open(url, '_blank')
  }
}

export function useTelegramUser(): TelegramUser | null {
  const [user, setUser] = useState<TelegramUser | null>(getTelegramUser())
  useEffect(() => setUser(getTelegramUser()), [])
  return user
}

export function useInitDataRaw(): string {
  const [raw, setRaw] = useState<string>(getInitDataRaw())
  useEffect(() => setRaw(getInitDataRaw()), [])
  return raw
}

export function useIsTelegramEnv(): boolean {
  const [v, setV] = useState<boolean>(isTelegramEnv())
  useEffect(() => setV(isTelegramEnv()), [])
  return v
}
```

- [ ] **Step 2: `web/src/lib/queryClient.ts`**

```ts
import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})
```

- [ ] **Step 3: `web/src/api/client.ts`**

```ts
import { getInitDataRaw } from '@/lib/telegram'
import { ApiEnvelope, ApiError } from '@/types/api'

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? ''

export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers)
  const raw = getInitDataRaw()
  if (raw) headers.set('Authorization', `tma ${raw}`)
  if (init?.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  let res: Response
  try {
    res = await fetch(`${BASE_URL}${path}`, { ...init, headers })
  } catch (e) {
    throw new ApiError(0, 'NETWORK', e instanceof Error ? e.message : 'network error')
  }

  let envelope: ApiEnvelope<T> | null = null
  try {
    envelope = (await res.json()) as ApiEnvelope<T>
  } catch {
    /* non-JSON body, leave envelope null */
  }

  if (!res.ok) {
    const code = envelope?.error?.code ?? 'UNKNOWN'
    const message = envelope?.error?.message ?? res.statusText
    throw new ApiError(res.status, code, message)
  }

  if (!envelope?.success || envelope.data === undefined) {
    throw new ApiError(res.status, 'PARSE', 'invalid response envelope')
  }

  return envelope.data
}
```

- [ ] **Step 4: `web/src/api/characters.ts`**

```ts
import { apiFetch } from '@/api/client'
import { Character } from '@/types/api'

export function listCharacters(): Promise<Character[]> {
  return apiFetch<Character[]>('/api/v1/characters')
}

export function getCharacter(id: string): Promise<Character> {
  return apiFetch<Character>(`/api/v1/characters/${encodeURIComponent(id)}`)
}
```

- [ ] **Step 5: `web/src/api/user.ts`**

```ts
import { apiFetch } from '@/api/client'
import { User, UserSettings } from '@/types/api'

export function me(): Promise<{ user: User; settings: UserSettings }> {
  return apiFetch<{ user: User; settings: UserSettings }>('/api/v1/me')
}

export function updateSettings(input: Partial<UserSettings>): Promise<UserSettings> {
  return apiFetch<UserSettings>('/api/v1/settings', {
    method: 'PATCH',
    body: JSON.stringify(input),
  })
}
```

- [ ] **Step 6: `web/src/api/subscription.ts`**

```ts
import { apiFetch } from '@/api/client'
import { Subscription } from '@/types/api'

export function getSubscription(): Promise<Subscription> {
  return apiFetch<Subscription>('/api/v1/subscription')
}
```

- [ ] **Step 7: `web/src/stores/userStore.ts`**

```ts
import { create } from 'zustand'
import type { TelegramUser } from '@/lib/telegram'
import type { UserSettings } from '@/types/api'

interface UserState {
  tgUser: TelegramUser | null
  settings: UserSettings | null
  setTgUser: (u: TelegramUser | null) => void
  setSettings: (s: UserSettings | null) => void
}

export const useUserStore = create<UserState>((set) => ({
  tgUser: null,
  settings: null,
  setTgUser: (tgUser) => set({ tgUser }),
  setSettings: (settings) => set({ settings }),
}))
```

- [ ] **Step 8: `web/src/stores/uiStore.ts`**

```ts
import { create } from 'zustand'

interface UiState {
  // Stub: grows as real UI features land.
  activeModal: string | null
  setActiveModal: (m: string | null) => void
}

export const useUiStore = create<UiState>((set) => ({
  activeModal: null,
  setActiveModal: (activeModal) => set({ activeModal }),
}))
```

- [ ] **Step 9: Typecheck**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run typecheck 2>&1 | tail -5
```
Expected: exits 0.

- [ ] **Step 10: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/lib web/src/api web/src/stores
git commit -m "feat(web): add telegram facade, query client, api layer, stores"
```

---

## Task 7: Components

**Files:**
- Create: `web/src/components/Layout.tsx`, `BottomNav.tsx`, `CharacterCard.tsx`, `ErrorBoundary.tsx`, `ErrorState.tsx`, `OutsideTelegramNotice.tsx`

- [ ] **Step 1: `web/src/components/OutsideTelegramNotice.tsx`**

```tsx
import { Placeholder } from '@telegram-apps/telegram-ui'

export function OutsideTelegramNotice() {
  return (
    <Placeholder
      header="Open this in Telegram"
      description="This app runs as a Telegram Mini App. Open it via the rolltonchatbot bot inside the Telegram client."
    />
  )
}
```

- [ ] **Step 2: `web/src/components/ErrorState.tsx`**

```tsx
import { Placeholder, Button } from '@telegram-apps/telegram-ui'
import { ApiError } from '@/types/api'

interface ErrorStateProps {
  error: unknown
  onRetry?: () => void
}

export function ErrorState({ error, onRetry }: ErrorStateProps) {
  const apiErr = error instanceof ApiError ? error : null
  const code = apiErr?.code ?? 'UNKNOWN'

  let header = 'Something went wrong'
  let description = apiErr?.message ?? (error instanceof Error ? error.message : 'Unknown error')
  let showRetry = true

  if (code === 'UNAUTHORIZED') {
    header = 'Open from Telegram'
    description = 'This screen needs Telegram authentication. Open the mini app via the bot.'
    showRetry = false
  } else if (code === 'NETWORK' || code === 'TIMEOUT') {
    header = 'Connection problem'
    description = "Couldn't reach the server. Check your connection and retry."
  }

  return (
    <Placeholder header={header} description={description}>
      {showRetry && onRetry ? <Button onClick={onRetry}>Retry</Button> : null}
    </Placeholder>
  )
}
```

- [ ] **Step 3: `web/src/components/ErrorBoundary.tsx`**

```tsx
import { Component, ReactNode } from 'react'
import { ErrorState } from '@/components/ErrorState'

interface Props {
  children: ReactNode
}
interface State {
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null }

  static getDerivedStateFromError(error: Error): State {
    return { error }
  }

  componentDidCatch(error: Error, info: { componentStack?: string | null }) {
    console.error('ErrorBoundary caught:', error, info)
  }

  render() {
    if (this.state.error) {
      return <ErrorState error={this.state.error} onRetry={() => window.location.reload()} />
    }
    return this.props.children
  }
}
```

- [ ] **Step 4: `web/src/components/BottomNav.tsx`**

```tsx
import { NavLink } from 'react-router-dom'
import { Tabbar } from '@telegram-apps/telegram-ui'
import { useState } from 'react'

const tabs = [
  { id: 'home', label: 'Home', path: '/' },
  { id: 'settings', label: 'Settings', path: '/settings' },
  { id: 'subscription', label: 'Plan', path: '/subscription' },
] as const

export function BottomNav() {
  const [current, setCurrent] = useState<string>('home')
  return (
    <Tabbar>
      {tabs.map((tab) => (
        <Tabbar.Item
          key={tab.id}
          selected={current === tab.id}
          onClick={() => setCurrent(tab.id)}
        >
          <NavLink to={tab.path}>{tab.label}</NavLink>
        </Tabbar.Item>
      ))}
    </Tabbar>
  )
}
```

- [ ] **Step 5: `web/src/components/CharacterCard.tsx`**

```tsx
import { Cell, Avatar } from '@telegram-apps/telegram-ui'
import { Character } from '@/types/api'

interface Props {
  character: Character
  onClick?: () => void
}

export function CharacterCard({ character, onClick }: Props) {
  return (
    <Cell
      before={<Avatar size={48} src={character.avatar_url} />}
      description={character.blurb}
      onClick={onClick}
    >
      {character.name}
    </Cell>
  )
}
```

- [ ] **Step 6: `web/src/components/Layout.tsx`**

```tsx
import { Outlet } from 'react-router-dom'
import { AppRoot } from '@telegram-apps/telegram-ui'
import { useIsTelegramEnv } from '@/lib/telegram'
import { BottomNav } from '@/components/BottomNav'
import { OutsideTelegramNotice } from '@/components/OutsideTelegramNotice'

export function Layout() {
  const inTelegram = useIsTelegramEnv()
  return (
    <AppRoot>
      <div className="flex h-full flex-col">
        <main className="flex-1 overflow-auto">
          {inTelegram ? <Outlet /> : <OutsideTelegramNotice />}
        </main>
        {inTelegram ? <BottomNav /> : null}
      </div>
    </AppRoot>
  )
}
```

- [ ] **Step 7: Typecheck**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run typecheck 2>&1 | tail -10
```
Expected: exits 0. If `Tabbar.Item` / `Cell` / `Avatar` prop signatures differ in the installed telegram-ui version, adjust to match the installed types — these components are stable in the public API but minor versions may rename props (e.g., `before` ↔ `slot="before"`). Use IDE/IDE typecheck output as the source of truth.

- [ ] **Step 8: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/components
git commit -m "feat(web): add layout, nav, character card, error UI components"
```

---

## Task 8: Pages

**Files:**
- Create: `web/src/pages/HomePage.tsx`, `CharacterPage.tsx`, `SettingsPage.tsx`, `SubscriptionPage.tsx`

- [ ] **Step 1: `web/src/pages/HomePage.tsx`**

```tsx
import { useQuery } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { Section, Spinner } from '@telegram-apps/telegram-ui'
import { listCharacters } from '@/api/characters'
import { CharacterCard } from '@/components/CharacterCard'
import { ErrorState } from '@/components/ErrorState'

export function HomePage() {
  const navigate = useNavigate()
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['characters'],
    queryFn: listCharacters,
  })

  if (isLoading) return <Spinner size="l" />
  if (error) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header="Characters">
      {data?.map((c) => (
        <CharacterCard
          key={c.id}
          character={c}
          onClick={() => navigate(`/characters/${c.id}`)}
        />
      ))}
    </Section>
  )
}
```

- [ ] **Step 2: `web/src/pages/CharacterPage.tsx`**

```tsx
import { useQuery } from '@tanstack/react-query'
import { useParams } from 'react-router-dom'
import { Section, Button, Spinner, Avatar } from '@telegram-apps/telegram-ui'
import { getCharacter } from '@/api/characters'
import { ErrorState } from '@/components/ErrorState'
import { openCharacterBot, useTelegramUser } from '@/lib/telegram'

export function CharacterPage() {
  const { id = '' } = useParams<{ id: string }>()
  const tgUser = useTelegramUser()
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['character', id],
    queryFn: () => getCharacter(id),
    enabled: !!id,
  })

  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header={data.name}>
      <div className="flex flex-col items-center gap-3 p-4">
        <Avatar size={96} src={data.avatar_url} />
        <p className="text-center">{data.blurb}</p>
        <Button
          size="l"
          onClick={() =>
            openCharacterBot(data.bot_username, `ref_${tgUser?.id ?? 'anon'}`)
          }
        >
          Open chat
        </Button>
      </div>
    </Section>
  )
}
```

- [ ] **Step 3: `web/src/pages/SettingsPage.tsx`**

```tsx
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Section, Cell, Switch, Spinner } from '@telegram-apps/telegram-ui'
import { me, updateSettings } from '@/api/user'
import { ErrorState } from '@/components/ErrorState'
import type { UserSettings } from '@/types/api'

export function SettingsPage() {
  const qc = useQueryClient()
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['me'],
    queryFn: me,
  })

  const mutation = useMutation({
    mutationFn: (patch: Partial<UserSettings>) => updateSettings(patch),
    onSuccess: (next) => {
      qc.setQueryData(['me'], (prev: { user: unknown; settings: UserSettings } | undefined) =>
        prev ? { ...prev, settings: next } : prev,
      )
    },
  })

  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header="Settings">
      <Cell
        after={
          <Switch
            checked={data.settings.notifications_enabled}
            onChange={(e) =>
              mutation.mutate({ notifications_enabled: e.target.checked })
            }
          />
        }
      >
        Notifications
      </Cell>
    </Section>
  )
}
```

- [ ] **Step 4: `web/src/pages/SubscriptionPage.tsx`**

```tsx
import { useQuery } from '@tanstack/react-query'
import { Section, Cell, Spinner } from '@telegram-apps/telegram-ui'
import { getSubscription } from '@/api/subscription'
import { ErrorState } from '@/components/ErrorState'

export function SubscriptionPage() {
  const { data, error, isLoading, refetch } = useQuery({
    queryKey: ['subscription'],
    queryFn: getSubscription,
  })

  if (isLoading) return <Spinner size="l" />
  if (error || !data) return <ErrorState error={error} onRetry={() => refetch()} />

  return (
    <Section header="Subscription">
      <Cell description={data.renews_at ? `Renews at ${data.renews_at}` : 'No active renewal'}>
        Plan: {data.tier}
      </Cell>
    </Section>
  )
}
```

- [ ] **Step 5: Typecheck**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run typecheck 2>&1 | tail -10
```
Expected: exits 0. Adjust telegram-ui component prop names if the installed version's types differ (same caveat as Task 7).

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/pages
git commit -m "feat(web): add home, character, settings, subscription pages"
```

---

## Task 9: Wire it all — `App.tsx`, `main.tsx`, `index.html`

**Files:**
- Create: `web/src/App.tsx`, `web/src/main.tsx`
- Modify: `web/index.html`

- [ ] **Step 1: Write `web/src/App.tsx`**

```tsx
import { createBrowserRouter, RouterProvider } from 'react-router-dom'
import { QueryClientProvider } from '@tanstack/react-query'
import { queryClient } from '@/lib/queryClient'
import { Layout } from '@/components/Layout'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { HomePage } from '@/pages/HomePage'
import { CharacterPage } from '@/pages/CharacterPage'
import { SettingsPage } from '@/pages/SettingsPage'
import { SubscriptionPage } from '@/pages/SubscriptionPage'

const router = createBrowserRouter([
  {
    path: '/',
    element: <Layout />,
    children: [
      { index: true, element: <HomePage /> },
      { path: 'characters/:id', element: <CharacterPage /> },
      { path: 'settings', element: <SettingsPage /> },
      { path: 'subscription', element: <SubscriptionPage /> },
    ],
  },
])

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ErrorBoundary>
        <RouterProvider router={router} />
      </ErrorBoundary>
    </QueryClientProvider>
  )
}
```

- [ ] **Step 2: Write `web/src/main.tsx`**

```tsx
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './styles/index.css'
import { initSDK } from '@/lib/telegram'
import { App } from '@/App'

initSDK()

const rootEl = document.getElementById('root')
if (!rootEl) throw new Error('root element not found')

createRoot(rootEl).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
```

- [ ] **Step 3: Update `web/index.html`**

Replace `web/index.html` with:
```html
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/favicon.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />
    <title>Rollton</title>
    <script src="https://telegram.org/js/telegram-web-app.js"></script>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```
Note: the `<script src="https://telegram.org/js/telegram-web-app.js">` is loaded synchronously in `<head>` so `window.Telegram.WebApp` is available before `main.tsx` runs `initSDK()`. This is the Telegram Mini App convention.

- [ ] **Step 4: Add `web/public/favicon.svg`**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
cat > public/favicon.svg <<'EOF'
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32"><circle cx="16" cy="16" r="14" fill="#3390ec"/><text x="16" y="22" font-family="sans-serif" font-size="16" font-weight="bold" fill="#fff" text-anchor="middle">R</text></svg>
EOF
```

- [ ] **Step 5: Build to verify everything compiles**

```bash
npm run build 2>&1 | tail -15
```
Expected: build exits 0; produces `dist/`.

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/App.tsx web/src/main.tsx web/index.html web/public/favicon.svg
git commit -m "feat(web): wire app, router, and telegram-web-app loader"
```

---

## Task 10: Vitest setup + smoke test (TDD)

**Files:**
- Modify: `web/src/test/setup.ts`
- Create: `web/src/test/HomePage.test.tsx`

- [ ] **Step 1: Write the failing test first**

Create `web/src/test/HomePage.test.tsx`:
```tsx
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { http, HttpResponse } from 'msw'
import { setupServer } from 'msw/node'
import { screen } from '@testing-library/react'
import { renderWithProviders } from '@/test/setup'
import { HomePage } from '@/pages/HomePage'

const server = setupServer(
  http.get('*/api/v1/characters', () =>
    HttpResponse.json({
      success: true,
      data: [
        { id: 'sherlock', name: 'Sherlock', blurb: 'Detective', bot_username: 'sherlock_bot' },
      ],
    }),
  ),
)

describe('HomePage', () => {
  beforeEach(() => server.listen())
  afterEach(() => server.resetHandlers())

  it('renders a character after loading', async () => {
    renderWithProviders(<HomePage />)
    expect(await screen.findByText('Sherlock')).toBeInTheDocument()
  })
})
```

- [ ] **Step 2: Run the test, watch it fail**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
npm run test 2>&1 | tail -10
```
Expected: FAIL — `renderWithProviders` is not exported from `@/test/setup` yet.

- [ ] **Step 3: Write `web/src/test/setup.ts`**

```ts
import '@testing-library/jest-dom/vitest'
import { ReactElement } from 'react'
import { render, RenderResult } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryRouter } from 'react-router-dom'
import { vi } from 'vitest'

// Stable fake Telegram WebApp so isTelegramEnv() returns true in tests.
vi.stubGlobal('Telegram', {
  WebApp: {
    initData: 'mock-init-data',
    initDataUnsafe: { user: { id: 42, first_name: 'Test', username: 'testuser' } },
    ready: () => {},
    expand: () => {},
    openTelegramLink: () => {},
  },
})

// One fresh QueryClient per render so tests don't share cache.
function freshClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: 0 } },
  })
}

export function renderWithProviders(ui: ReactElement): RenderResult {
  return render(
    <QueryClientProvider client={freshClient()}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>,
  )
}
```

(The `setup.tsx` file was created in Task 3 Step 3 with the right extension; the vite config already references `./src/test/setup.tsx`. Write to that file directly.)

- [ ] **Step 4: Run the test, watch it pass**

```bash
npm run test 2>&1 | tail -10
```
Expected: PASS — 1 test, 1 passed.

- [ ] **Step 5: Verify lint and typecheck still clean**

```bash
npm run typecheck && npm run lint
```
Expected: both exit 0.

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/src/test
git commit -m "test(web): add vitest setup and HomePage smoke test"
```

---

## Task 11: Env files + README + final scripts polish

**Files:**
- Create: `web/.env.example`, `web/.env.development`, `web/README.md`
- Modify: `web/.gitignore`

- [ ] **Step 1: Write `web/.env.example`**

```dotenv
# Backend base URL — must be reachable from the browser running the mini app.
VITE_API_BASE_URL=http://localhost:8080

# Main bot username — used by deep-link helpers in lib/telegram.ts.
VITE_BOT_USERNAME=rolltonchatbot
```

- [ ] **Step 2: Write `web/.env.development`**

```dotenv
VITE_API_BASE_URL=http://localhost:8080
VITE_BOT_USERNAME=rolltonchatbot
```

- [ ] **Step 3: Ensure `web/.gitignore` exists with the right entries**

Open `web/.gitignore` (created by Vite template). Verify these lines are present; add if missing:
```gitignore
node_modules
dist
.env.local
.env.*.local
coverage
*.log
.vite
```

- [ ] **Step 4: Write `web/README.md`**

```markdown
# web/

Telegram Mini App for the `rolltonchatbot` (main bot). React + TypeScript + Vite, Tailwind v4, Zustand + TanStack Query, telegram-ui.

## Prerequisites

- Node 20+
- The rolltonchatbot Go backend running locally (`make -C ../bot run BOT=rolltonchatbot`) for API calls to succeed. The stub will render but show errors until the backend implements the endpoints in Section 9 of the spec.

## First-time setup

```bash
cp .env.example .env.local           # adjust as needed; .env.local is gitignored
npm install
```

## Scripts

```bash
npm run dev          # vite dev server (default http://localhost:5173)
npm run build        # production build into dist/
npm run preview      # serve the prod bundle locally
npm run test         # vitest one-shot (CI mode)
npm run test:watch   # vitest watch mode
npm run lint         # eslint
npm run format       # prettier --write .
npm run typecheck    # tsc --noEmit
```

## Telegram-flavored dev

Mini apps must be served over HTTPS to load inside Telegram. For local dev:

1. Run `npm run dev` (Vite serves http://localhost:5173).
2. Expose it via a tunnel: `cloudflared tunnel --url http://localhost:5173`.
3. Tell @BotFather to set the mini app URL to the HTTPS tunnel URL (`/setmenubutton` → web app URL).
4. Open the bot in Telegram and tap the menu button.

For pure component dev, just open `http://localhost:5173/` — `<Layout>` renders `<OutsideTelegramNotice>` and stops further routing.

## Tech notes

- Telegram glue is in `src/lib/telegram.ts` as a thin facade over `window.Telegram.WebApp`. `index.html` loads `telegram-web-app.js` synchronously so the global is available before React mounts.
- Auth header: `Authorization: tma <initData>` — the rolltonchatbot Go backend validates HMAC.
- Server state in TanStack Query; client state in Zustand. Don't put server cache in Zustand.
- Tailwind v4 uses CSS-first config; there is no `tailwind.config.ts`. Customize via `src/styles/index.css` `@theme` block when needed.
```

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add web/.env.example web/.env.development web/.gitignore web/README.md
git commit -m "docs(web): env files and README"
```

---

## Task 12: End-to-end verification (no commit)

- [ ] **Step 1: Fresh build**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/web
rm -rf dist
npm run build
```
Expected: `dist/index.html`, `dist/assets/*.{js,css}` present; build exits 0.

- [ ] **Step 2: Tests**

```bash
npm run test
```
Expected: 1 file, 1 test passed.

- [ ] **Step 3: Lint + format check + typecheck**

```bash
npm run lint && npm run format:check && npm run typecheck
```
Expected: all exit 0.

- [ ] **Step 4: Dev server smoke test**

Run dev in one terminal:
```bash
npm run dev
```
In another terminal:
```bash
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:5173/
```
Expected: `200`. (The page is the SPA shell; full hydration happens in browser, but the static response should be served.)

Stop dev:
```bash
# back in the dev terminal, press Ctrl-C
```

- [ ] **Step 5: Prod preview smoke test**

```bash
npm run preview &
sleep 2
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:4173/
kill %1 2>/dev/null
```
Expected: `200`.

- [ ] **Step 6: Final clean tree**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git status
```
Expected: working tree clean.

---

## Out-of-scope (intentional, deferred)

- The Zustand stores (`userStore`, `uiStore`) are created but no page reads from them in the stub. Pages talk to TanStack Query directly. The stores are scaffolded so future feature code has a place to put hydrated client state.
- No backend endpoints implemented — `/api/v1/*` calls return 404 until the rolltonchatbot Go service grows them.
- No Telegram HMAC validation middleware on the Go side (the spec section 9 lists it; implement in a separate plan).
- No CI workflow.
- No deployment target chosen.
- No analytics / Sentry.
- No i18n.
- No PWA / service worker.
- No real product features (chat, subscription checkout, etc.).

## Open items resolved at execution time

- The exact prop signatures of `@telegram-apps/telegram-ui` components (`Cell`, `Tabbar`, `Switch`) may differ between minor versions. If `npm run typecheck` complains about a prop in Tasks 7–8, adjust to match the installed `dist/types`. The component names themselves are stable.
- If the Vite template installs Tailwind v3 by default in the future (unlikely, but possible), uninstall it and install `tailwindcss@^4` + `@tailwindcss/vite` per Task 2.
