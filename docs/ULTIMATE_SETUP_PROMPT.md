# Ultimate Setup Prompt

Paste the section below into a fresh Claude (or similar AI assistant) session to bootstrap a Rollton-shaped monorepo from scratch: Go Telegram bots with hex-arch, a Vite/React Telegram Mini App, Terraform on AWS, and CI on GitHub Actions.

It captures the conventions and decisions made in this codebase. It is **not** a script — it's an instruction set that walks the assistant through the same workflow (brainstorm → spec → plan → execute) you used here.

---

## How to use it

1. Create a new empty directory for the new monorepo.
2. Open a fresh assistant session inside it.
3. Paste **everything** between the `=== BEGIN PROMPT ===` and `=== END PROMPT ===` markers below.
4. Fill in the values inside the `<<< ... >>>` placeholders at the top of the prompt before pasting.
5. Drive the assistant by replying "go" / approving designs / pushing through each `brainstorm → spec → plan → execute` cycle.

---

=== BEGIN PROMPT ===

# Goal

You will scaffold a Telegram-native product monorepo with three deployables: Go bots (`bot/`), a Vite/React Telegram Mini App (`web/`), and Terraform infrastructure (`infra/`). Follow the workflow and conventions below **exactly**. Defer to the user at every approval gate.

## Project specifics (fill these in)

| Placeholder | Value |
|---|---|
| Product description | <<< one-line product pitch, e.g. "Character.AI-style chat product hosted entirely on Telegram" >>> |
| Main bot username | <<< e.g. `myproductbot` >>> |
| Admin bot username | <<< e.g. `myproduct_admin_bot` >>> |
| GitHub repo URL | <<< e.g. `git@github.com:owner/repo.git` >>> |
| GitHub repo slug | <<< e.g. `owner/repo` >>> |
| Domain root | <<< e.g. `myproduct.com` >>> |
| AWS region | <<< e.g. `eu-central-1` >>> |
| Project name | <<< e.g. `myproduct` (used as `name_prefix` for AWS resources) >>> |

## Architecture decisions (do not negotiate)

1. **Monorepo layout**: `bot/` (Go), `web/` (Vite/React TS), `infra/` (Terraform), `docs/superpowers/{specs,plans}/` (workflow artifacts).
2. **Bot stack**: Go 1.22+, hex-arch (`cmd/internal/pkg/db`), goose migrations, sqlc with `pgx/v5`, pgxpool, slog, `golang.org/x/sync/errgroup`. Long-poll only (no webhook). `github.com/go-telegram-bot-api/telegram-bot-api/v5`. `github.com/google/uuid` for IDs.
3. **Web stack**: Vite 7 + React 18 + TypeScript, Zustand (client state) + TanStack Query (server state), `@telegram-apps/telegram-ui` + Tailwind v4 (CSS-first config), React Router 7 data-mode, Vitest + happy-dom + MSW. Pin React to 18 because telegram-ui peer-deps it.
4. **Cloud**: AWS EC2 `t4g.small` on-demand (NOT Spot — bots need 24/7 uptime), RDS `db.t4g.micro`, Cloudflare Pages (frontend, free), Cloudflare DNS, Caddy + Let's Encrypt on the EC2 for `api.<domain>` HTTPS. SSM Parameter Store for runtime secrets. State backend: S3 with `use_lockfile = true` (Terraform ≥ 1.10, NO DynamoDB).
5. **CI**: GitHub Actions, one workflow per artifact (`web.yml`, `bot.yml`, `infra.yml`), path-filtered. AWS auth via OIDC role assumption (no static keys). Apply gated behind GitHub `environment: production` for manual approval. GHCR for Docker images. Cloudflare Pages deploys via its native Git integration (NOT from CI).
6. **Service pattern** (this is the most important convention — follow the **viza_assigment pattern**, not the typical "ports & adapters" overkill):
   - `internal/core/domain/<entity>.go` holds the type, **all sentinel errors**, constructors (`NewX(...)`), and invariants/match functions (`(x).Matches(y)`).
   - `internal/core/ports/services.go` holds **service** interfaces (not repository interfaces — there is no repository layer).
   - `internal/core/services/<entity>_service.go` is `type xService struct { queries *postgres.Queries }`, constructed via `NewXService(db postgres.DBTX) ports.XService`. Holds the sqlc-generated `*Queries` directly. Translates DB errors inline via `errors.Join(errors.New("context"), err)` and `errors.Is(err, pgx.ErrNoRows)`.
   - `pkg/sqlc/postgres/translate.go` holds hand-written `ToDomainX`, `StringToText`, `TextOrEmpty` helpers — the only adapter-layer code that exists.
   - **Cross-bot** services live in `internal/core/services/`. **Bot-specific** services live in `internal/bots/<name>/services/`.
   - Tests use **pgxmock** (pgx equivalent of viza's `go-sqlmock`).
7. **Telegram glue**: thin facade in `pkg/tgbot/` (Bot, Router, Context, Middleware, Chain). Context exposes `SetCtx(ctx)` and `NewTestContext(...)`. Middleware-driven user auto-registration via `EnsureUserRegistered` (silent — only writes to ctx, never replies). Admin bot adds `AllowOnlyUserIDs(ids, log)` middleware after auth. Run `index.html` loads `https://telegram.org/js/telegram-web-app.js` synchronously so `window.Telegram.WebApp` is populated before React mounts.
8. **Mini-app auth wire format**: `Authorization: tma <initDataRaw>`. Frontend pulls `initDataRaw` from `window.Telegram.WebApp.initData`. Backend will eventually validate HMAC via `github.com/telegram-mini-apps/init-data-golang` (out of scope for initial scaffold).

## Workflow

Use the brainstorm → spec → plan → execute cycle for **every** non-trivial change. Specs at `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`, plans at `docs/superpowers/plans/YYYY-MM-DD-<topic>.md`. Commit each artifact before proceeding.

For each cycle:
1. **Brainstorm**: ask clarifying questions one at a time, propose 2–3 approaches, lead with a recommendation.
2. **Spec**: write the design doc, self-review for placeholders / contradictions / scope, ask the user to approve.
3. **Plan**: enumerate tasks with exact file paths, complete code blocks, exact commands, expected output, frequent commits. TDD where it fits (services, middleware, handlers). No placeholders.
4. **Execute**: batch tasks, checkpoint at boundaries, run fmt/lint/test as you go.

## Execution order

Do these in order. Each is its own brainstorm→spec→plan→execute cycle.

### 1. Bot scaffold

Empty Go scaffold. No domain code yet.

- `bot/go.mod`, `bot/Dockerfile` (multi-stage, ARG BOT), `bot/docker-compose.yml` (postgres + bot services), `bot/Makefile` (build/run/test/lint/migrate/sqlc/schema-dump), `bot/.air.toml`, `bot/.env.example`, `bot/README.md`, `bot/sqlc.yaml`, `bot/db/schema/schema.sql` (empty placeholder), `bot/scripts/schema-dump.sh`.
- Layout:
  - `bot/cmd/<main-bot>/main.go`, `bot/cmd/admin/main.go`
  - `bot/internal/bots/<main-bot>/{wire.go, handlers/{telegram,http}/, services/}`
  - `bot/internal/bots/admin/{wire.go, handlers/{telegram,http}/, services/}`
  - `bot/internal/core/{domain,ports,services}` (each with `test/`)
  - `bot/internal/{config,middleware,testutil}`
  - `bot/pkg/{http,tgbot,sqlc/postgres}` (last one empty until first sqlc gen)
  - `bot/db/{migrations,queries,schema}`
- `pkg/tgbot/`: `bot.go` (Bot, Run long-poll), `router.go` (Handle, HandleCallback, HandleDefault, Dispatch), `context.go` (Context, Ctx, **SetCtx**, **NewTestContext**, Reply, ReplyMarkdown), `middleware.go` (Middleware, Chain).
- `pkg/http/`: `response.go` (WriteJSON, WriteSuccess, WriteCreated, WriteError using `net/http` — NOT gin), `errors.go` (error code constants).
- `internal/config/config.go`: env loader with godotenv, validates `DATABASE_URL` and `TELEGRAM_TOKEN`.
- `internal/middleware/logging.go`: slog-based, exports both `Telegram(log)` (tgbot middleware) and `HTTP(log)` (`func(http.Handler) http.Handler`).
- First commit: bootstrap monorepo with spec + plan.

### 2. Web mini-app scaffold

- `npm create vite@latest web -- --template react-ts`. Pin Vite to `^7` if Node < 20.19 (Vite 8 + rolldown require ≥20.19).
- Pin React to `^18.3` (telegram-ui peer-requires React 18).
- Deps:
  - Runtime: `react-router-dom@^7`, `zustand@^5`, `@tanstack/react-query@^5`, `@telegram-apps/sdk-react`, `@telegram-apps/telegram-ui`.
  - Dev: `tailwindcss@^4`, `@tailwindcss/vite`, `@types/node`, `vitest`, `@testing-library/{react,jest-dom,user-event}`, `happy-dom` (NOT jsdom — jsdom's css-calc dep breaks on Node 20.18), `msw`, `prettier`, `eslint-config-prettier`.
- `vite.config.ts` imports `defineConfig` from `'vitest/config'` (NOT `'vite'`), registers `react()` + `tailwindcss()`, `@` alias to `./src`. Test env `happy-dom`, `css: false`, `setupFiles: ['./src/test/setup.tsx']`.
- `src/styles/index.css`: `@import "tailwindcss"; @import "@telegram-apps/telegram-ui/dist/styles.css";`
- `tsconfig.app.json`: add `"baseUrl": "."`, `"paths": {"@/*": ["src/*"]}`, `"ignoreDeprecations": "6.0"` (for TS 6's `baseUrl` deprecation), `"types": ["vite/client", "vitest/globals"]`.
- Telegram facade in `src/lib/telegram.ts`: thin wrapper over `window.Telegram.WebApp` (do NOT use `@telegram-apps/sdk-react` hooks directly — facade decouples from SDK version churn). Exports `initSDK()`, `getInitDataRaw()`, `getTelegramUser()`, `isTelegramEnv()`, `openCharacterBot(username, payload)`, plus `useTelegramUser()` / `useInitDataRaw()` / `useIsTelegramEnv()` hooks that return values directly (no `useState + useEffect` — the script loads synchronously).
- `src/types/telegram.d.ts`: minimal ambient typing for `window.Telegram.WebApp`.
- `src/api/client.ts`: `apiFetch<T>(path, init)`, injects `Authorization: tma <initDataRaw>`, unwraps `{success, data, error}` envelope, throws typed `ApiError`.
- `src/test/setup.tsx`: mocks `window.Telegram` with `initData`, `initDataUnsafe.user`, **plus** `colorScheme`, `themeParams`, `platform`, `version`, `onEvent`, `offEvent` (last two required by telegram-ui's `AppRoot`). `renderWithProviders` wraps in `<QueryClientProvider>` + `<AppRoot>` + `<MemoryRouter>`.
- `index.html`: include `<script src="https://telegram.org/js/telegram-web-app.js"></script>` in `<head>`.
- `package.json` scripts: `dev`, `build` (`tsc -b && vite build`), `preview`, `test` (`vitest run`), `test:watch`, `lint`, `format`, `format:check`, `typecheck` (`tsc -b --noEmit`). `"engines": {"node": ">=20"}`.
- Components: `Layout` (uses `useIsTelegramEnv` to short-circuit to `<OutsideTelegramNotice>` outside Telegram), `BottomNav` (Telegram-ui `Tabbar` + react-router `NavLink`), `CharacterCard`, `ErrorBoundary` (class component), `ErrorState` (code-aware: UNAUTHORIZED / NETWORK / TIMEOUT / UNKNOWN), `OutsideTelegramNotice`.
- Pages: `HomePage` (useQuery characters), `CharacterPage` (open-chat deep link via `openCharacterBot`), `SettingsPage` (useQuery me + useMutation updateSettings), `SubscriptionPage`.
- Smoke test only — one `HomePage.test.tsx` that uses MSW to mock `/api/v1/characters` and asserts cards render.

### 3. Infra scaffold (Terraform)

- `infra/{Makefile, README.md, .gitignore}`.
- `infra/bootstrap/`: one-shot module that creates the S3 state bucket (versioned + encrypted + public-access blocked). Local state. Run once manually.
- `infra/envs/prod/`: composes six modules. `backend.tf` has prominent `// ─── THIS IS TERRAFORM STATE BACKEND, NOT THE BOT API BACKEND ───` callout because the term collision is confusing.
- Modules:
  - `network`: VPC, 2 public subnets across 2 AZs, IGW, route table. No NAT (saves $33/mo; nothing needs private outbound).
  - `bot_host`: EC2 (Amazon Linux 2023 ARM64), Elastic IP, SG (SSH from operator CIDR, 80+443 from world), IAM role with SSM read on prefix + KMS decrypt scoped to `kms:ViaService = ssm`, keypair, IMDSv2 only. `user_data.sh.tftpl` installs Docker + docker-compose plugin + Caddy (with `Caddyfile` for `api.<domain>` → `localhost:8080`), writes a systemd unit that fetches SSM SecureStrings into `/opt/rollton/.env.<bot>` on boot, plus a `docker compose up -d --pull always` unit. `lifecycle { ignore_changes = [ami] }` so AMI drift doesn't recreate.
  - `database`: RDS Postgres 16, db.t4g.micro, gp3 20GB, single-AZ, `random_password`, `skip_final_snapshot = true` (flip after first revenue). Egress: bot host SG only.
  - `secrets`: SSM SecureStrings. `database_url` is set by Terraform; per-bot `telegram_token` parameters are created with placeholder values and `lifecycle { ignore_changes = [value] }` so operator can rotate via `aws ssm put-parameter --overwrite` without Terraform fighting back.
  - `dns`: Cloudflare records — `api.<domain>` A-record to EIP (proxied=false; Caddy handles TLS), `app.<domain>` CNAME to `<project>.pages.dev` (proxied=true).
  - `github_oidc`: OpenID Connect provider for GitHub Actions, IAM role trusted on `repo:<owner>/<repo>:ref:refs/heads/main`. AttachAdministratorAccess initially; tighten later.
- Cloudflare Pages is **NOT** Terraformed — set up once in the Cloudflare dashboard with Git integration (Production branch: `main`, Root directory: `web`, Build command: `npm ci && npm run build`, Output: `dist`).
- `.github/workflows/`:
  - `web.yml`: lint + typecheck + test + build only (Cloudflare Pages deploys separately).
  - `bot.yml`: test → build multi-platform Docker images → push to GHCR → SSH to EC2 → `docker compose pull && up -d`.
  - `infra.yml`: `fmt -check` → `init` → `validate` → `plan` (comment on PR). Apply job gated by `environment: production`.

### 4. First feature: Telegram user auto-registration

This is the first vertical slice through the stack. Use the viza_assigment pattern strictly.

- Migration: `users` table with `id UUID PRIMARY KEY DEFAULT gen_random_uuid()`, `telegram_id BIGINT NOT NULL UNIQUE`, name/lang/premium fields, `created_at/updated_at TIMESTAMPTZ`. Goose-annotated.
- sqlc queries: `GetUserByTelegramID :one`, `UpsertUserFromTelegram :one` (`ON CONFLICT (telegram_id) DO UPDATE`).
- Domain (`internal/core/domain/user.go`): `User` struct, `ErrUserNotFound`, `func NewUser(telegramID, username, firstName, lastName, languageCode string, isPremium bool) User`, `func (u User) TelegramFieldsMatch(other User) bool`.
- Port (`internal/core/ports/services.go`): `UserService` interface with `EnsureRegistered(ctx, user domain.User) (domain.User, error)`.
- Service (`internal/core/services/user_service.go`): `type userService struct { queries *postgres.Queries }`, `NewUserService(db postgres.DBTX) ports.UserService`. Logic:
  ```go
  stored, err := s.queries.GetUserByTelegramID(ctx, user.TelegramID)
  if err != nil && !errors.Is(err, pgx.ErrNoRows) {
      return domain.User{}, errors.Join(errors.New("getting user"), err)
  }
  if err == nil {
      existing := postgres.ToDomainUser(stored)
      if existing.TelegramFieldsMatch(user) { return existing, nil }
  }
  // ... upsert ...
  ```
- Translate helpers (`pkg/sqlc/postgres/translate.go`): `ToDomainUser`, `StringToText`, `TextOrEmpty`.
- Tests with **pgxmock** (`go get github.com/pashagolub/pgxmock/v4`). 4 cases: found-matching skips upsert, drift triggers upsert, ErrNoRows triggers upsert, other error bubbles via `errors.Is`.
- Middleware (`internal/middleware/auth.go`):
  - `type UserService interface { EnsureRegistered(ctx, user domain.User) (domain.User, error) }` (declared here, NOT imported from services — loosens coupling).
  - `userCtxKey struct{}`, `WithUser`, `UserFromContext`.
  - `EnsureUserRegistered(svc, log)`: skip when `SentFrom() == nil`; build `domain.NewUser(...)`; call svc; on error log and pass through; on success `c.SetCtx(WithUser(c.Ctx(), &u))`.
- Allowlist middleware (`internal/middleware/allowlist.go`): `AllowOnlyUserIDs([]int64, log)` — silent drop with INFO log for non-allowed senders. Empty list = reject all.
- Wire: middleware order in admin's `b.Use(...)` is `Telegram(log)` → `EnsureUserRegistered(svc, log)` → `AllowOnlyUserIDs(allowed, log)`. Order matters: admins still get registered into `users`, but only allowlisted IDs reach handlers.
- Both bots: `cmd/<bot>/main.go` opens `pgxpool.New(ctx, cfg.DB.URL)`, `userSvc := services.NewUserService(pool)` — NO separate repo construction. Wire `Deps.UserSvc ports.UserService`.
- `/start` handler personalized via `middleware.UserFromContext(c.Ctx())`.
- Admin allowlist parsed in `cmd/admin/main.go` from `ADMIN_ALLOWED_USER_IDS` env (comma-separated int64), passed via `Deps.AllowedUserIDs []int64`.

## Hard rules (catch yourself before breaking these)

- **Errors live in `domain/`**, not next to the code that throws them. One sentinel per domain concept.
- **No repository abstraction.** The service IS the data layer. `*postgres.Queries` is its dependency.
- **Service constructors take `postgres.DBTX`** (the sqlc-emitted interface), NOT `*pgxpool.Pool` directly — this is what makes pgxmock work in tests.
- **No `useState + useEffect` for synchronous globals** in the web app. `window.Telegram.WebApp` is populated before React mounts; hooks just return live values.
- **Push directly to main** only at the user's request and only after the user has explicitly authorized it.
- **Never commit secrets.** `.env`, `.env.*` are gitignored. Tokens go in SSM Parameter Store (production) or per-bot `.env.<botname>` files (gitignored, local dev only).
- **Tailwind v4 = no `tailwind.config.ts`** unless you specifically need to extend the theme. CSS-first config.
- **Vite 7 with Node < 20.19**, Vite 8 with Node ≥ 20.19. Picking wrong here gives confusing rolldown native-binding errors.
- **happy-dom in tests, not jsdom.** jsdom's css-calc transitive dep breaks Node 20.18's stricter ESM rules.
- **Spot EC2 is wrong for bots.** Bots need 24/7 uptime; Spot can be reclaimed. Use on-demand.
- **DynamoDB is no longer needed for Terraform state locking** (Terraform ≥ 1.10 has native S3 locking via `use_lockfile = true`).
- **Cloudflare Pages is not Terraformed.** Set it up once in the dashboard, point it at the repo.

## VS Code debug

Create `.vscode/launch.json` at repo root with three configs: "Debug `<main-bot>`" (envFile `bot/.env.<main-bot>`), "Debug admin" (envFile `bot/.env.admin`), "Debug current Go test" (`mode: test`, `program: ${file}`). Both env files gitignored; the bot env files contain `TELEGRAM_TOKEN=...` + `DATABASE_URL` + ports.

## Common deferred items (call these out in spec/plan as out-of-scope)

- TMA initData HMAC validation middleware for the Go HTTP server (separate spec).
- Multi-AZ RDS.
- Auto-scaling.
- WAF.
- Per-character bots (one Telegram bot per character — same Go binary template, separate cmd/ entry).
- Subscription checkout flow.
- i18n.
- PWA / service worker for the mini app.

## When the user pushes

After they say "push", `git push -u origin main` (or just `git push origin main` if upstream set). Sanity-grep tracked files for any leaked bot token prefix before pushing (specific to your Telegram token's bot ID).

=== END PROMPT ===

---

## Notes for the human reader (NOT pasted into the next session)

- This prompt assumes the assistant has the Superpowers skill set (brainstorming, writing-plans, executing-plans, finishing-a-development-branch). If not, the workflow still applies — just enforce it manually.
- The exact dependencies and versions in this codebase as of 2026-06-13 are pinned in `bot/go.mod`, `web/package.json`, and `infra/envs/prod/providers.tf`. Bump as appropriate when reusing.
- The repo at https://github.com/kotovconst/rollton is the reference implementation. Open any of these files in the new session if you need a concrete example: `bot/internal/core/services/user_service.go`, `bot/pkg/sqlc/postgres/translate.go`, `web/src/lib/telegram.ts`, `infra/modules/bot_host/user_data.sh.tftpl`.
