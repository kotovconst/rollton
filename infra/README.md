# infra/

Terraform configuration for Rollton AWS resources + DNS, plus the GitHub Actions workflows that gate `terraform plan`/`apply`.

See `docs/superpowers/specs/2026-06-13-rollton-infra-design.md` for the design.

## What lives where (and what "backend" means here)

The word "backend" is overloaded. To prevent confusion:

| Component | Where it runs | Terraform handles it? |
|---|---|---|
| **Bot API backend** (Go service, `api.rollton.com`) | EC2 t4g.small | yes — `modules/bot_host` |
| Postgres database | RDS db.t4g.micro | yes — `modules/database` |
| Mini app frontend (`app.rollton.com`) | Cloudflare Pages | no — set up once in the Cloudflare dashboard |
| DNS records | Cloudflare | yes — `modules/dns` |
| **Terraform state** (bookkeeping JSON Terraform writes) | S3 bucket | created by `bootstrap/`, referenced via `backend "s3"` in `envs/prod/backend.tf` |

The `backend "s3"` block in `envs/prod/backend.tf` is **Terraform's own state storage**, not the bot's API backend. They're the same word for two unrelated things.

## Layout

| Dir | Purpose |
|---|---|
| `bootstrap/` | One-shot module that creates the S3 state bucket. Run **once**, manually, with local state. |
| `envs/prod/` | The main configuration. Uses S3 backend. Run `plan`/`apply` here. |
| `modules/` | Reusable modules composed by `envs/prod/`. |

## Prerequisites

- Terraform `>= 1.10`
- AWS credentials with admin on the target account
- Cloudflare API token with `Zone:DNS:Edit` on `rollton.com`
- The Cloudflare zone for `rollton.com` already created in the Cloudflare dashboard (Terraform references it by ID; it does not create the zone)

## First-time bootstrap

```bash
cd infra/bootstrap
terraform init
terraform apply -auto-approve   # writes bucket name to local terraform.tfstate (gitignored)
```

Copy the bucket name into `envs/prod/backend.tf` (already wired by default).

## Routine operation

```bash
cd infra/envs/prod
cp terraform.tfvars.example terraform.tfvars   # edit secrets / IDs
terraform init                                  # reads S3 backend
terraform plan
terraform apply
```

In CI, `infra.yml` does this automatically with manual approval gated by GitHub's `environment: production`.

## End-to-end first-time setup runbook

The order matters — some steps depend on outputs of earlier steps.

### 0. Prerequisites

- AWS account, admin IAM user with access keys configured locally (for bootstrap + first apply)
- Cloudflare account, domain `rollton.com` added to it with nameservers set at the registrar
- Cloudflare API token with `Zone:DNS:Edit` on the zone
- SSH key pair on your machine (`ssh-keygen -t ed25519 -C operator@rollton`)
- The `rolltonchatbot/admin` Docker images pushed to GHCR via `bot.yml` workflow at least once (or build & push manually)

### 1. Bootstrap state bucket

```bash
cd infra/bootstrap
terraform init
terraform apply -var "state_bucket_name=rollton-tfstate-<YOUR_AWS_ACCOUNT_ID>"
```

Copy the bucket name into `infra/envs/prod/backend.tf` (replace the `rollton-tfstate` placeholder).

### 2. First apply of the main config

```bash
cd infra/envs/prod
cp terraform.tfvars.example terraform.tfvars   # fill values
terraform init                                 # uses S3 backend now
terraform apply
```

### 3. Populate manually-set SSM parameters

```bash
aws ssm put-parameter --region eu-central-1 --overwrite \
  --name /rollton/prod/rolltonchatbot/telegram_token \
  --type SecureString \
  --value "1234567890:REPLACE_WITH_BOTFATHER_TOKEN"

aws ssm put-parameter --region eu-central-1 --overwrite \
  --name /rollton/prod/admin/telegram_token \
  --type SecureString \
  --value "..."
```

Then SSH in and trigger a re-fetch:

```bash
ssh ec2-user@$(terraform output -raw bot_host_public_ip)
sudo systemctl start rollton-env.service
sudo systemctl restart rollton-compose.service
```

### 4. Configure GitHub repo secrets (used by workflows)

- `AWS_OIDC_ROLE_ARN`: from `terraform output github_actions_role_arn`
- `EC2_HOST`: from `terraform output bot_host_public_ip`
- `EC2_SSH_KEY`: contents of your private key (`~/.ssh/id_ed25519`, no passphrase)
- `CLOUDFLARE_API_TOKEN`: the API token used locally

### 5. Configure Cloudflare Pages (one-time)

Cloudflare dashboard → Pages → "Connect to Git" → select `kotovconst/rollton`:

- Production branch: `main`
- Root directory: `web`
- Build command: `npm ci && npm run build`
- Output directory: `dist`
- Environment variable: `NODE_VERSION=20`

After the first deploy, note the `*.pages.dev` URL and update `cloudflare_pages_cname` in `terraform.tfvars`; re-apply.

### 6. Tell Telegram where the mini app is

`@BotFather → /setmenubutton → rolltonchatbot → https://app.rollton.com → "Open Rollton"`.

### 7. Verify

- `curl https://api.rollton.com/healthz` should return the JSON envelope.
- Open `rolltonchatbot` in Telegram, tap the menu button — the mini app should load.
