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
