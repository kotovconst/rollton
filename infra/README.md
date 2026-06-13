# infra/

Terraform configuration for Rollton AWS resources + DNS, plus the GitHub Actions workflows that gate `terraform plan`/`apply`.

See `docs/superpowers/specs/2026-06-13-rollton-infra-design.md` for the design.

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
