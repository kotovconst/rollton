# Rollton Infrastructure (`infra/`) — design

**Date:** 2026-06-13
**Status:** Approved (design phase)
**Scope:** Terraform configuration for AWS resources hosting `bot/`, plus the CI/CD workflows for `bot/`, `web/`, and `infra/`. Cloudflare Pages hosts `web/` outside of Terraform.

## 1. Context

Rollton is a pre-revenue Character.AI clone on Telegram. The monorepo has three deployables:

- **`bot/`** — Go binaries (`rolltonchatbot`, `admin`, plus future per-character bots). Long-poll only; no inbound public traffic except a small REST API (`/healthz`, future `/api/v1/*`) that the mini app calls.
- **`web/`** — Vite static SPA. Telegram requires HTTPS for `web_app_url`.
- **`infra/`** — this spec. Manages AWS resources for the bot and Postgres. The mini app's hosting is outside Terraform (Cloudflare Pages via its Git integration).

Hard constraints:
- Pre-revenue → cost must be minimal.
- One engineer → ops complexity must be minimal.
- Bots must stay up 24/7 → no Spot, no aggressive scale-to-zero.
- Mini app must serve HTTPS on a custom domain.
- Bot's API endpoint must serve HTTPS (Telegram-issued initData auth happens through it).

## 2. Decisions

| Decision | Choice | Rationale |
|---|---|---|
| Cloud | AWS for compute + DB, Cloudflare for DNS + static hosting | Cheapest reasonable combo |
| Region | `eu-central-1` (Frankfurt) | Close to expected EU/RU audience |
| Compute | Single EC2 `t4g.small` on-demand, ARM | ~$12/mo, fits 10+ Go bots in 2 GB RAM |
| DB | RDS `db.t4g.micro` single-AZ | ~$15/mo; managed backups, easier than self-hosting Postgres on the EC2 |
| Container orchestration | Docker Compose on the EC2 | Same compose file we already write for dev |
| Container registry | GitHub Container Registry (GHCR) | Free with the repo |
| Frontend hosting | Cloudflare Pages | Free; Telegram-compatible HTTPS; no Terraform needed |
| DNS | Cloudflare | Free; both apex and subdomains |
| TLS for bot API | Caddy on the EC2 with Let's Encrypt | Free, automatic, no ACM needed |
| Secrets at runtime | AWS Systems Manager Parameter Store (`/rollton/...`) | Free for standard params; KMS-encrypted for SecureString |
| Secrets in CI | GitHub Actions OIDC → IAM role assumption | No long-lived AWS keys in GitHub |
| Terraform state | S3 backend with native S3 locking (`use_lockfile = true`, Terraform ≥ 1.10) | Bootstrap step (chicken-and-egg with Terraform); no DynamoDB needed |
| Environments | Single `prod` only for now | YAGNI; add `staging` when scale demands it |
| Multi-AZ | No (RDS subnet group spans two AZs because RDS requires it, but RDS itself is single-AZ) | Cost; downtime tolerance is hours, not minutes |
| Backups | RDS automated backups, 7-day retention | Default; sufficient for pre-revenue |

## 3. Repository layout

```
rollton/
├── bot/                                # already scaffolded
├── web/                                # scaffolded separately
├── infra/
│   ├── README.md                       # overview + bootstrap instructions
│   ├── Makefile                        # init/plan/apply/destroy shortcuts
│   ├── .gitignore                      # *.tfstate*, .terraform/, etc.
│   ├── bootstrap/                      # one-shot module that creates the TF state bucket + lock table
│   │   ├── main.tf
│   │   ├── variables.tf
│   │   ├── outputs.tf
│   │   └── README.md
│   ├── envs/
│   │   └── prod/                       # the only env for now
│   │       ├── backend.tf              # S3 backend config
│   │       ├── main.tf                 # composes modules
│   │       ├── variables.tf
│   │       ├── outputs.tf
│   │       ├── providers.tf            # aws + cloudflare provider declarations
│   │       └── terraform.tfvars.example
│   └── modules/
│       ├── network/                    # VPC, 2 public subnets, IGW, route table
│       │   ├── main.tf  variables.tf  outputs.tf
│       ├── bot_host/                   # EC2 + EIP + SG + IAM instance profile + user_data
│       │   ├── main.tf  variables.tf  outputs.tf
│       │   └── user_data.sh.tftpl
│       ├── database/                   # RDS Postgres + subnet group + SG
│       │   ├── main.tf  variables.tf  outputs.tf
│       ├── dns/                        # Cloudflare DNS records for api.rollton.com (Pages handles app.)
│       │   ├── main.tf  variables.tf  outputs.tf
│       ├── secrets/                    # SSM Parameter Store entries
│       │   ├── main.tf  variables.tf  outputs.tf
│       └── github_oidc/                # IAM role trusted by GitHub Actions OIDC
│           ├── main.tf  variables.tf  outputs.tf
└── .github/
    └── workflows/
        ├── web.yml                     # web/ CI (build/test only; CF Pages deploys)
        ├── bot.yml                     # bot/ test + docker push + ssh deploy
        └── infra.yml                   # infra/ plan-on-PR + apply-on-main
```

### Layout rationale

- **`envs/prod/`** is the root module Terraform applies. It composes the six `modules/`. Adding `staging` later is `cp -r prod staging`.
- **`modules/`** are reusable; each takes flat inputs and emits flat outputs. No deep nesting.
- **`bootstrap/`** is separate because it creates the *state backend* — chicken-and-egg if it shared state with the main config. Run once, manually, with local state, then commit nothing of its `terraform.tfstate` (gitignored; the bucket lives on, the local state is throwaway).

## 4. AWS resources (per module)

### 4.1 `modules/network`
- 1× VPC (`10.20.0.0/16`)
- 2× public subnets (`10.20.1.0/24` in `eu-central-1a`, `10.20.2.0/24` in `eu-central-1b`)
- 1× Internet Gateway
- 1× route table with `0.0.0.0/0 → igw`, associated with both subnets

No NAT, no private subnets — saves $33/mo, acceptable because nothing in this stack needs private outbound except RDS (and RDS doesn't need outbound).

### 4.2 `modules/bot_host`
- 1× EC2 `t4g.small`, AMI: latest Amazon Linux 2023 ARM64
- 1× Elastic IP, associated
- 1× security group:
  - Ingress: 22/tcp from operator IP CIDR (variable), 80/tcp + 443/tcp from `0.0.0.0/0`
  - Egress: all
- 1× SSH key pair (public key passed in via variable)
- 1× IAM role + instance profile with permissions: `ssm:GetParameter*` on `/rollton/*`, `cloudwatch:PutMetricData`
- 1× `user_data` script that:
  1. Installs Docker + docker-compose plugin
  2. Installs and runs the SSM agent (preinstalled on AL2023)
  3. Creates `/opt/rollton`, `chown` to `ec2-user`
  4. Writes `/opt/rollton/docker-compose.yml` (the prod variant — references `image:`, not `build:`)
  5. Writes `/opt/rollton/Caddyfile` (TLS reverse proxy for `api.rollton.com → :8080`)
  6. Pulls `.env` values from SSM at boot via a systemd unit, writes to `/opt/rollton/.env`
  7. `docker compose pull && docker compose up -d`

### 4.3 `modules/database`
- 1× RDS Postgres 16 (`db.t4g.micro`)
- 1× DB subnet group across both public subnets
- 1× security group: ingress 5432/tcp from bot_host SG, no public access
- Storage: 20 GB gp3
- Backup retention: 7 days
- Maintenance window: weekly off-peak
- `publicly_accessible = false`
- Master password: random, stored in SSM Parameter Store

### 4.4 `modules/dns`
- Provider: `cloudflare/cloudflare`
- `cloudflare_record` for `api.rollton.com` → EIP, proxied = false (Caddy on EC2 does TLS via Let's Encrypt; Cloudflare proxy not needed)
- `cloudflare_record` for `app.rollton.com` → Cloudflare Pages target (CNAME to `<project>.pages.dev`), proxied = true
- Zone is pre-created in Cloudflare (manual one-time step) — Terraform references it by zone ID

### 4.5 `modules/secrets`
- `aws_ssm_parameter` (SecureString) for:
  - `/rollton/prod/database_url` — `postgres://...` constructed from RDS outputs
  - `/rollton/prod/rolltonchatbot/telegram_token` — populated manually after first apply
  - `/rollton/prod/admin/telegram_token` — populated manually after first apply
- `lifecycle { ignore_changes = [value] }` on the Telegram tokens so manual rotation doesn't fight Terraform

### 4.6 `modules/github_oidc`
- `aws_iam_openid_connect_provider` for `token.actions.githubusercontent.com`
- `aws_iam_role` trusted by that provider, scoped to `repo:kotovconst/rollton:ref:refs/heads/main`
- Policy: `AdministratorAccess` for now (tighten later). Permissions only relevant for `infra.yml` workflow.

## 5. CI/CD pipelines

Three workflows, each scoped by `paths:` filter:

### 5.1 `.github/workflows/web.yml`
- Triggers: PR + push to `main`, only when `web/**` changes
- Jobs: install → lint → typecheck → test → build
- **No deploy step** — Cloudflare Pages handles deploy via its Git integration
- Purpose: fail fast on broken code before CF Pages tries

### 5.2 `.github/workflows/bot.yml`
- Triggers: PR + push to `main`, only when `bot/**` changes
- `test` job (always): `go vet`, `go test -race ./...`
- `build-and-deploy` job (only on push to `main`):
  1. Login to GHCR
  2. `docker buildx build --build-arg BOT=rolltonchatbot --push -t ghcr.io/kotovconst/rollton-rolltonchatbot:{latest, ${{sha}}}`
  3. Same for `admin`
  4. SSH to EC2:
     - `docker compose pull`
     - `docker compose run --rm rolltonchatbot goose -dir /app/db/migrations postgres "$DATABASE_URL" up`
     - `docker compose up -d --remove-orphans`
     - `docker image prune -f`
- Secrets used: `EC2_HOST`, `EC2_SSH_KEY` (private key matching the keypair Terraform attached). `GITHUB_TOKEN` is auto-provided for GHCR.

### 5.3 `.github/workflows/infra.yml`
- Triggers: PR + push to `main`, only when `infra/**` changes
- Auth to AWS: OIDC role assumption (no static AWS keys)
- Auth to Cloudflare: `CLOUDFLARE_API_TOKEN` repo secret, exported as `TF_VAR_cloudflare_api_token` (or equivalent env var the provider reads) on both `plan` and `apply` jobs
- `plan` job (always): `fmt -check`, `init`, `validate`, `plan`. Plan output commented on PRs.
- `apply` job (only on push to `main`): protected by GitHub `environment: production` → requires manual approval click in the Actions UI. Inherits the same auth env vars as `plan`.

## 6. State backend (chicken-and-egg)

Terraform state lives in S3 with **native S3 locking** (Terraform ≥ 1.10's `use_lockfile = true`). No DynamoDB table needed — S3 conditional writes handle concurrent-apply protection. The S3 bucket can't be created by the same Terraform that uses it as a backend, so the bootstrap step is separate:

1. **Manual one-time bootstrap:**
   ```
   cd infra/bootstrap
   terraform init                  # local state
   terraform apply                 # creates S3 bucket (versioned, encrypted)
   ```
   Output: bucket name. Stored in the operator's notes and in `infra/envs/prod/backend.tf`.

2. **Main config uses it via backend:**
   ```
   cd infra/envs/prod
   terraform init                  # reads backend.tf → S3 (with use_lockfile = true)
   terraform plan / apply          # reads/writes state in S3
   ```

Bootstrap's own `terraform.tfstate` is gitignored. The bucket itself outlives any future re-bootstrap.

Required versions: `terraform >= 1.10`, AWS provider `>= 5.50`.

## 7. Data flow & deploy

### 7.1 Steady-state runtime
```
Telegram user ─► Cloudflare DNS ─┬─► app.rollton.com (Pages) ────► static SPA assets
                                  │
                                  └─► api.rollton.com (DNS-only) ─► EIP ─► Caddy (TLS)
                                                                     │
                                                                     ▼
                                                              Docker Compose
                                                              ├── rolltonchatbot
                                                              ├── admin
                                                              └── (future char bots)
                                                                     │
                                                                     ▼
                                                              RDS Postgres
                                                              (VPC-private)
```

### 7.2 Bot deploy flow
```
git push main
   │  bot/** changed
   ▼
GHA: go test
   │
   ▼
GHA: docker build (multi-platform: linux/arm64) & push to GHCR
   │
   ▼
GHA: ssh ec2-user@EIP
      docker compose pull
      docker compose run --rm rolltonchatbot goose up   # migration
      docker compose up -d
```

### 7.3 Frontend deploy flow
```
git push main
   │  web/** changed
   ▼
GHA: lint/typecheck/test (gate; no deploy)
   │
   ▼
Cloudflare Pages (independent of GHA, polls the repo)
   ├── npm ci && npm run build
   └── deploys dist/ to app.rollton.com
```

### 7.4 Infra deploy flow
```
git push main
   │  infra/** changed
   ▼
GHA assume-role via OIDC
   │
   ▼
GHA: terraform plan
   │
   ▼
GitHub "environment: production" gate ── manual approval
   │
   ▼
GHA: terraform apply
```

## 8. Secrets handling

| Secret | Where it lives | Who sets the value |
|---|---|---|
| RDS master password | SSM SecureString `/rollton/prod/db/master_password` | Terraform (random_password) |
| `DATABASE_URL` | SSM SecureString `/rollton/prod/database_url` | Terraform (composed from RDS outputs + master_password) |
| `TELEGRAM_TOKEN` per bot | SSM SecureString `/rollton/prod/<bot>/telegram_token` | Manually after first apply (Terraform ignores value changes) |
| GitHub Actions → AWS | OIDC role assumption | No static secret stored |
| GitHub Actions → SSH to EC2 | `EC2_SSH_KEY` GitHub repo secret | Operator pastes the private key once |
| GitHub Actions → GHCR | `GITHUB_TOKEN` auto-provided | Built into Actions |
| Cloudflare API token (for `cloudflare` Terraform provider) | `CLOUDFLARE_API_TOKEN` GitHub Actions secret | Created manually in Cloudflare dashboard |

The EC2 fetches `.env` values from SSM at boot via a small `rollton-env.service` systemd unit (writes to `/opt/rollton/.env`, sets `EnvironmentFile` for the compose unit).

## 9. Telegram registration (manual, one-time)

Not in Terraform — no provider exists. After first deploy:
1. `@BotFather → /setmenubutton → rolltonchatbot → https://app.rollton.com → "Open Rollton"`
2. Verify by opening rolltonchatbot in Telegram and tapping the menu button.

## 10. Out-of-scope (deferred)

- Staging environment (add when needed)
- Multi-AZ failover for RDS (cost; pre-revenue)
- Auto-scaling (single-EC2 is sufficient for current scale)
- WAF (Cloudflare provides basic protection for the SPA; bot API is low-volume)
- Bastion host (SSH direct to EC2 via key + restricted CIDR is fine for now)
- CloudWatch dashboards + alarms (add when running into actual incidents)
- Backup automation beyond RDS defaults
- DR / cross-region replication
- Cost alerts (recommended to set manually in AWS Budgets; not in this Terraform)
- Cloudflare Pages Terraform management (uses dashboard Git integration instead)
- ACM certificates (Caddy + Let's Encrypt handles TLS on the bot host; CF Pages handles its own)
- ECR (using GHCR instead)
- ALB (Caddy on the EC2 serves as the entry point; saves $22/mo)

## 11. Costs (estimated, idle)

| Item | $/mo |
|---|---|
| EC2 `t4g.small` on-demand | ~$12 |
| EBS gp3 30 GB (root) | ~$2.40 |
| RDS `db.t4g.micro` | ~$13 |
| RDS storage 20 GB gp3 | ~$2.30 |
| RDS backups (within retention) | $0 |
| Elastic IP (attached) | $0 |
| SSM Parameter Store (standard) | $0 |
| Data transfer egress | ~$0–1 (Telegram long-poll is mostly inbound) |
| Cloudflare Pages | $0 |
| Cloudflare DNS | $0 |
| GHCR | $0 (within repo storage quota) |
| GitHub Actions minutes | $0 (within free tier for private repo, with ~5 min builds) |
| **Total** | **~$30/mo** |

## 12. Open items resolved at execution time

- Cloudflare account ID + zone ID for `rollton.com` (or whatever domain is registered).
- Operator IP CIDR for SSH security group ingress.
- SSH public key to attach to the EC2 key pair.
- AWS account ID (for the GitHub Actions OIDC IAM role trust policy).
- Whether `rollton.com` is already registered in Cloudflare / Route53 / elsewhere. Spec assumes Cloudflare; adjust DNS module if otherwise.
- Whether to enable Cloudflare proxy ("orange cloud") on `api.rollton.com`. Spec says no (Caddy direct). Switching later is a single attribute change.
