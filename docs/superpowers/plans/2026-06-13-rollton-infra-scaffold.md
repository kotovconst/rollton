# Rollton Infrastructure Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Scaffold `infra/` with Terraform that creates AWS resources (EC2 bot host + RDS + SSM + IAM + DNS via Cloudflare) and add three GitHub Actions workflows (`web.yml`, `bot.yml`, `infra.yml`). Apply is deferred to execution time; this plan produces code that `terraform fmt` and `terraform validate` cleanly.

**Architecture:** Six reusable modules (`network`, `bot_host`, `database`, `secrets`, `dns`, `github_oidc`) composed by a single `envs/prod/` root module. State in S3 with native S3 locking (Terraform ≥ 1.10, no DynamoDB). Bootstrap is a separate one-shot module run with local state to create the state bucket. Cloudflare Pages hosts the SPA outside Terraform.

**Tech Stack:** Terraform `>= 1.10`, AWS provider `>= 5.50`, Cloudflare provider `~> 4`, random provider `~> 3.6`. GitHub Actions with AWS OIDC (no static AWS keys). Caddy on EC2 for Let's Encrypt TLS.

**Reference spec:** `docs/superpowers/specs/2026-06-13-rollton-infra-design.md`.

---

## Pre-flight values (substituted at execution time)

| Placeholder | Default | Where used |
|---|---|---|
| `<AWS_REGION>` | `eu-central-1` | `envs/prod/terraform.tfvars` |
| `<AWS_ACCOUNT_ID>` | (12-digit account number) | `github_oidc` trust policy, output formatting |
| `<DOMAIN_ROOT>` | `rollton.com` | DNS records — `api.<DOMAIN_ROOT>`, `app.<DOMAIN_ROOT>` |
| `<CF_ACCOUNT_ID>` | (Cloudflare account ID) | Cloudflare provider |
| `<CF_ZONE_ID>` | (Cloudflare zone ID for `rollton.com`) | DNS module |
| `<CF_PAGES_CNAME>` | `<project>.pages.dev` | DNS app record |
| `<GHCR_OWNER>` | `kotovconst` | EC2 user_data, GHA workflows |
| `<GITHUB_REPO>` | `kotovconst/rollton` | OIDC role trust policy |
| `<SSH_PUBLIC_KEY>` | (operator's id_ed25519.pub contents) | EC2 key pair |
| `<OPERATOR_IP_CIDR>` | `0.0.0.0/0` (loosened in stub; tighten to operator's home IP /32) | SSH ingress |

These all become `terraform.tfvars` values, not committed.

---

## File Structure

```
infra/
├── README.md
├── Makefile
├── .gitignore
├── bootstrap/
│   ├── README.md
│   ├── main.tf            # S3 state bucket (versioned, encrypted)
│   ├── variables.tf
│   ├── outputs.tf
│   └── providers.tf
├── envs/
│   └── prod/
│       ├── backend.tf     # S3 backend (use_lockfile = true)
│       ├── providers.tf   # aws + cloudflare + random
│       ├── variables.tf   # root-level inputs
│       ├── main.tf        # composes modules
│       ├── outputs.tf     # surface what GHA / operator needs
│       ├── locals.tf      # shared computed values
│       └── terraform.tfvars.example
└── modules/
    ├── network/    { main.tf, variables.tf, outputs.tf }
    ├── bot_host/   { main.tf, variables.tf, outputs.tf, user_data.sh.tftpl }
    ├── database/   { main.tf, variables.tf, outputs.tf }
    ├── secrets/    { main.tf, variables.tf, outputs.tf }
    ├── dns/        { main.tf, variables.tf, outputs.tf }
    └── github_oidc/{ main.tf, variables.tf, outputs.tf }

.github/
└── workflows/
    ├── web.yml
    ├── bot.yml
    └── infra.yml
```

Validation cadence: `terraform fmt -recursive` after each task. Full `terraform init -backend=false && terraform validate` runs only once at the end (Task 11), inside `envs/prod/`.

---

## Task 1: `infra/` root + bootstrap module

**Files:**
- Create: `infra/.gitignore`, `infra/README.md`, `infra/Makefile`
- Create: `infra/bootstrap/{README.md, providers.tf, variables.tf, main.tf, outputs.tf}`
- Delete: `infra/README.md` placeholder created during bot scaffolding (rewriting)

- [ ] **Step 1: `infra/.gitignore`**

```gitignore
# Terraform
.terraform/
.terraform.lock.hcl
*.tfstate
*.tfstate.*
*.tfplan
crash.log
crash.*.log
override.tf
override.tf.json
*_override.tf
*_override.tf.json

# Inputs (may contain secrets)
*.tfvars
!*.tfvars.example
```

- [ ] **Step 2: `infra/README.md`** (overwrites placeholder)

```markdown
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
```

- [ ] **Step 3: `infra/Makefile`**

```make
.PHONY: help fmt fmt-check validate plan apply destroy bootstrap-init bootstrap-apply

ENV ?= prod
ENV_DIR := envs/$(ENV)

help:
	@echo "Targets:"
	@echo "  bootstrap-init   - terraform init in bootstrap/ (local state)"
	@echo "  bootstrap-apply  - terraform apply in bootstrap/ (one-shot)"
	@echo "  fmt              - terraform fmt -recursive across infra/"
	@echo "  fmt-check        - terraform fmt -check -recursive"
	@echo "  validate         - terraform validate in envs/\$$ENV (default: prod)"
	@echo "  plan             - terraform plan in envs/\$$ENV"
	@echo "  apply            - terraform apply in envs/\$$ENV"
	@echo "  destroy          - terraform destroy in envs/\$$ENV"

fmt:
	terraform fmt -recursive .

fmt-check:
	terraform fmt -check -recursive .

validate:
	cd $(ENV_DIR) && terraform init -backend=false -input=false && terraform validate

plan:
	cd $(ENV_DIR) && terraform plan

apply:
	cd $(ENV_DIR) && terraform apply

destroy:
	cd $(ENV_DIR) && terraform destroy

bootstrap-init:
	cd bootstrap && terraform init

bootstrap-apply:
	cd bootstrap && terraform apply
```

- [ ] **Step 4: `infra/bootstrap/providers.tf`**

```hcl
terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.50"
    }
  }
}

provider "aws" {
  region = var.aws_region
}
```

- [ ] **Step 5: `infra/bootstrap/variables.tf`**

```hcl
variable "aws_region" {
  description = "AWS region for the state bucket"
  type        = string
  default     = "eu-central-1"
}

variable "state_bucket_name" {
  description = "Globally unique S3 bucket name for Terraform state. Suggested: rollton-tfstate-<account-id>"
  type        = string
}

variable "tags" {
  description = "Common resource tags"
  type        = map(string)
  default = {
    Project   = "rollton"
    ManagedBy = "terraform"
    Component = "bootstrap"
  }
}
```

- [ ] **Step 6: `infra/bootstrap/main.tf`**

```hcl
resource "aws_s3_bucket" "tfstate" {
  bucket = var.state_bucket_name
  tags   = var.tags
}

resource "aws_s3_bucket_versioning" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_server_side_encryption_configuration" "tfstate" {
  bucket = aws_s3_bucket.tfstate.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "tfstate" {
  bucket                  = aws_s3_bucket.tfstate.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
```

- [ ] **Step 7: `infra/bootstrap/outputs.tf`**

```hcl
output "state_bucket_name" {
  description = "S3 bucket name for the main Terraform state. Paste into envs/<env>/backend.tf."
  value       = aws_s3_bucket.tfstate.id
}

output "state_bucket_arn" {
  value = aws_s3_bucket.tfstate.arn
}
```

- [ ] **Step 8: `infra/bootstrap/README.md`**

```markdown
# bootstrap/

One-shot Terraform module that creates the S3 bucket for the main configuration's state.

This module uses **local state** because it's chicken-and-egg with the bucket it creates.

## Run once

```bash
cd infra/bootstrap
terraform init
terraform apply -var "state_bucket_name=rollton-tfstate-<account-id>"
```

Output: `state_bucket_name`. Paste into `infra/envs/prod/backend.tf` (it's the value already templated there).

The local `terraform.tfstate` is gitignored. You can lose it without consequence — the bucket itself survives, and you can re-import via `terraform import` if you ever need to manage it again.
```

- [ ] **Step 9: Format**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
```
Expected: no output (already formatted).

- [ ] **Step 10: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/
git commit -m "feat(infra): bootstrap module for terraform state bucket"
```

---

## Task 2: `envs/prod/` skeleton

**Files:**
- Create: `infra/envs/prod/{backend.tf, providers.tf, variables.tf, locals.tf, main.tf, outputs.tf, terraform.tfvars.example}`

- [ ] **Step 1: `infra/envs/prod/backend.tf`**

```hcl
terraform {
  backend "s3" {
    bucket       = "rollton-tfstate"
    key          = "envs/prod/terraform.tfstate"
    region       = "eu-central-1"
    encrypt      = true
    use_lockfile = true
  }
}
```
Note: the `bucket` value here must match what Task 1 step 6 created via `bootstrap`. The operator updates this string after running bootstrap; the default `rollton-tfstate` is a placeholder.

- [ ] **Step 2: `infra/envs/prod/providers.tf`**

```hcl
terraform {
  required_version = ">= 1.10"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.50"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = local.common_tags
  }
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
```

- [ ] **Step 3: `infra/envs/prod/variables.tf`**

```hcl
variable "aws_region" {
  description = "AWS region for all resources"
  type        = string
  default     = "eu-central-1"
}

variable "name_prefix" {
  description = "Prefix for resource names"
  type        = string
  default     = "rollton-prod"
}

variable "operator_ip_cidr" {
  description = "CIDR allowed to SSH into the bot host (your home/office /32)"
  type        = string
}

variable "ssh_public_key" {
  description = "Public key (OpenSSH format) for the bot host key pair"
  type        = string
}

variable "domain_root" {
  description = "Apex domain managed by Cloudflare (e.g. rollton.com)"
  type        = string
}

variable "cloudflare_api_token" {
  description = "Cloudflare API token with Zone:DNS:Edit on the domain"
  type        = string
  sensitive   = true
}

variable "cloudflare_zone_id" {
  description = "Cloudflare zone ID for the domain"
  type        = string
}

variable "cloudflare_pages_cname" {
  description = "Cloudflare Pages target CNAME (e.g. rollton-web.pages.dev)"
  type        = string
}

variable "github_repo" {
  description = "GitHub repo slug for OIDC trust policy (owner/repo)"
  type        = string
  default     = "kotovconst/rollton"
}

variable "ghcr_owner" {
  description = "GHCR namespace owner — used to construct image URLs"
  type        = string
  default     = "kotovconst"
}

variable "bot_instance_type" {
  description = "EC2 instance type for the bot host"
  type        = string
  default     = "t4g.small"
}

variable "db_instance_class" {
  description = "RDS instance class"
  type        = string
  default     = "db.t4g.micro"
}

variable "db_allocated_storage" {
  description = "RDS storage in GB"
  type        = number
  default     = 20
}
```

- [ ] **Step 4: `infra/envs/prod/locals.tf`**

```hcl
locals {
  common_tags = {
    Project     = "rollton"
    Environment = "prod"
    ManagedBy   = "terraform"
  }

  api_domain = "api.${var.domain_root}"
  app_domain = "app.${var.domain_root}"

  ssm_prefix = "/rollton/prod"

  # Bots running on the host. Adding one is one more entry here + a new SSM token.
  bots = [
    { name = "rolltonchatbot", port = 8080, tag = "latest" },
    { name = "admin", port = 8081, tag = "latest" },
  ]

  azs = [
    "${var.aws_region}a",
    "${var.aws_region}b",
  ]
}
```

- [ ] **Step 5: `infra/envs/prod/main.tf`** (skeleton — modules wired in Task 9)

```hcl
# Modules are wired in `Task 9`. This file is intentionally light until then.

data "aws_caller_identity" "current" {}
```

- [ ] **Step 6: `infra/envs/prod/outputs.tf`** (empty for now)

```hcl
# Outputs populated in Task 9 after modules are wired.

output "aws_account_id" {
  value = data.aws_caller_identity.current.account_id
}
```

- [ ] **Step 7: `infra/envs/prod/terraform.tfvars.example`**

```hcl
# Copy to terraform.tfvars and fill in real values.

operator_ip_cidr       = "203.0.113.42/32"
ssh_public_key         = "ssh-ed25519 AAAA... operator@example"
domain_root            = "rollton.com"
cloudflare_api_token   = "cf_pat_..."
cloudflare_zone_id     = "abc123..."
cloudflare_pages_cname = "rollton-web.pages.dev"
```

- [ ] **Step 8: Format**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
```

- [ ] **Step 9: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/envs/
git commit -m "feat(infra): envs/prod skeleton with backend, providers, locals"
```

---

## Task 3: `modules/network`

**Files:**
- Create: `infra/modules/network/{main.tf, variables.tf, outputs.tf}`

- [ ] **Step 1: `infra/modules/network/variables.tf`**

```hcl
variable "name_prefix" {
  type = string
}

variable "vpc_cidr" {
  type    = string
  default = "10.20.0.0/16"
}

variable "public_subnet_cidrs" {
  type    = list(string)
  default = ["10.20.1.0/24", "10.20.2.0/24"]
}

variable "azs" {
  type        = list(string)
  description = "Two AZs to spread public subnets across"
}

variable "tags" {
  type    = map(string)
  default = {}
}
```

- [ ] **Step 2: `infra/modules/network/main.tf`**

```hcl
resource "aws_vpc" "this" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true
  tags                 = merge(var.tags, { Name = "${var.name_prefix}-vpc" })
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id
  tags   = merge(var.tags, { Name = "${var.name_prefix}-igw" })
}

resource "aws_subnet" "public" {
  count                   = length(var.public_subnet_cidrs)
  vpc_id                  = aws_vpc.this.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = var.azs[count.index]
  map_public_ip_on_launch = true
  tags = merge(var.tags, {
    Name = "${var.name_prefix}-public-${count.index}"
  })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = merge(var.tags, { Name = "${var.name_prefix}-public" })
}

resource "aws_route_table_association" "public" {
  count          = length(aws_subnet.public)
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}
```

- [ ] **Step 3: `infra/modules/network/outputs.tf`**

```hcl
output "vpc_id" {
  value = aws_vpc.this.id
}

output "public_subnet_ids" {
  value = aws_subnet.public[*].id
}
```

- [ ] **Step 4: Format**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
```

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/modules/network/
git commit -m "feat(infra): network module — VPC, 2 public subnets, IGW"
```

---

## Task 4: `modules/database`

**Files:**
- Create: `infra/modules/database/{main.tf, variables.tf, outputs.tf}`

- [ ] **Step 1: `infra/modules/database/variables.tf`**

```hcl
variable "name_prefix" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "subnet_ids" {
  type = list(string)
}

variable "bot_security_group_id" {
  description = "Security group ID of the bot host — only source allowed to connect"
  type        = string
}

variable "instance_class" {
  type    = string
  default = "db.t4g.micro"
}

variable "allocated_storage" {
  type    = number
  default = 20
}

variable "engine_version" {
  type    = string
  default = "16.4"
}

variable "db_name" {
  type    = string
  default = "rollton"
}

variable "master_username" {
  type    = string
  default = "rollton"
}

variable "tags" {
  type    = map(string)
  default = {}
}
```

- [ ] **Step 2: `infra/modules/database/main.tf`**

```hcl
resource "random_password" "master" {
  length  = 32
  special = false # avoid characters that need URL-encoding
}

resource "aws_db_subnet_group" "this" {
  name       = "${var.name_prefix}-db"
  subnet_ids = var.subnet_ids
  tags       = var.tags
}

resource "aws_security_group" "db" {
  name        = "${var.name_prefix}-db"
  description = "Postgres access — bot host only"
  vpc_id      = var.vpc_id
  tags        = var.tags
}

resource "aws_vpc_security_group_ingress_rule" "db_from_bot" {
  security_group_id            = aws_security_group.db.id
  referenced_security_group_id = var.bot_security_group_id
  ip_protocol                  = "tcp"
  from_port                    = 5432
  to_port                      = 5432
  description                  = "Postgres from bot host"
}

resource "aws_db_instance" "this" {
  identifier             = "${var.name_prefix}-pg"
  engine                 = "postgres"
  engine_version         = var.engine_version
  instance_class         = var.instance_class
  allocated_storage      = var.allocated_storage
  storage_type           = "gp3"
  storage_encrypted      = true
  db_name                = var.db_name
  username               = var.master_username
  password               = random_password.master.result
  db_subnet_group_name   = aws_db_subnet_group.this.name
  vpc_security_group_ids = [aws_security_group.db.id]
  publicly_accessible    = false
  multi_az               = false

  backup_retention_period   = 7
  backup_window             = "02:00-03:00"
  maintenance_window        = "Sun:03:30-Sun:04:30"
  copy_tags_to_snapshot     = true
  skip_final_snapshot       = true # pre-revenue convenience; flip to false later
  final_snapshot_identifier = null

  apply_immediately          = false
  auto_minor_version_upgrade = true

  tags = merge(var.tags, { Name = "${var.name_prefix}-pg" })
}
```

- [ ] **Step 3: `infra/modules/database/outputs.tf`**

```hcl
output "endpoint" {
  value = aws_db_instance.this.address
}

output "port" {
  value = aws_db_instance.this.port
}

output "db_name" {
  value = aws_db_instance.this.db_name
}

output "master_username" {
  value = aws_db_instance.this.username
}

output "master_password" {
  value     = random_password.master.result
  sensitive = true
}

output "connection_string" {
  value     = "postgres://${aws_db_instance.this.username}:${random_password.master.result}@${aws_db_instance.this.address}:${aws_db_instance.this.port}/${aws_db_instance.this.db_name}?sslmode=require"
  sensitive = true
}
```

- [ ] **Step 4: Format**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
```

- [ ] **Step 5: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/modules/database/
git commit -m "feat(infra): database module — RDS postgres + sg + random master pwd"
```

---

## Task 5: `modules/bot_host`

**Files:**
- Create: `infra/modules/bot_host/{main.tf, variables.tf, outputs.tf, user_data.sh.tftpl}`

- [ ] **Step 1: `infra/modules/bot_host/variables.tf`**

```hcl
variable "name_prefix" {
  type = string
}

variable "aws_region" {
  type = string
}

variable "vpc_id" {
  type = string
}

variable "subnet_id" {
  type = string
}

variable "instance_type" {
  type    = string
  default = "t4g.small"
}

variable "ssh_public_key" {
  type = string
}

variable "operator_ip_cidr" {
  type = string
}

variable "ssm_param_prefix" {
  type        = string
  description = "SSM Parameter Store path prefix the instance is allowed to read"
}

variable "api_domain" {
  type        = string
  description = "Public hostname the bot's HTTPS API is served at (e.g. api.rollton.com)"
}

variable "ghcr_owner" {
  type        = string
  description = "GHCR namespace (the part between ghcr.io/ and /image)"
}

variable "bots" {
  type = list(object({
    name = string
    port = number
    tag  = string
  }))
  description = "Bots to run on the host. Each becomes a compose service."
}

variable "root_volume_size_gb" {
  type    = number
  default = 30
}

variable "tags" {
  type    = map(string)
  default = {}
}
```

- [ ] **Step 2: `infra/modules/bot_host/main.tf`**

```hcl
data "aws_ami" "al2023_arm" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-*-arm64"]
  }
  filter {
    name   = "architecture"
    values = ["arm64"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

resource "aws_security_group" "this" {
  name        = "${var.name_prefix}-bot"
  description = "Bot host — SSH (operator) + HTTP/HTTPS public"
  vpc_id      = var.vpc_id
  tags        = var.tags
}

resource "aws_vpc_security_group_ingress_rule" "ssh" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = var.operator_ip_cidr
  ip_protocol       = "tcp"
  from_port         = 22
  to_port           = 22
  description       = "SSH from operator"
}

resource "aws_vpc_security_group_ingress_rule" "http" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = 80
  to_port           = 80
  description       = "HTTP (Caddy ACME challenge)"
}

resource "aws_vpc_security_group_ingress_rule" "https" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "tcp"
  from_port         = 443
  to_port           = 443
  description       = "HTTPS"
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_iam_role" "this" {
  name = "${var.name_prefix}-bot"
  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
      Action = "sts:AssumeRole"
    }]
  })
  tags = var.tags
}

resource "aws_iam_role_policy" "ssm_read" {
  name = "ssm-read"
  role = aws_iam_role.this.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
          "ssm:GetParameters",
          "ssm:GetParametersByPath"
        ]
        Resource = "arn:aws:ssm:${var.aws_region}:*:parameter${var.ssm_param_prefix}/*"
      },
      {
        Effect = "Allow"
        Action = ["kms:Decrypt"]
        Resource = "*"
        Condition = {
          StringEquals = {
            "kms:ViaService" = "ssm.${var.aws_region}.amazonaws.com"
          }
        }
      }
    ]
  })
}

resource "aws_iam_instance_profile" "this" {
  name = "${var.name_prefix}-bot"
  role = aws_iam_role.this.name
}

resource "aws_key_pair" "this" {
  key_name   = "${var.name_prefix}-bot"
  public_key = var.ssh_public_key
}

resource "aws_instance" "this" {
  ami                    = data.aws_ami.al2023_arm.id
  instance_type          = var.instance_type
  subnet_id              = var.subnet_id
  vpc_security_group_ids = [aws_security_group.this.id]
  iam_instance_profile   = aws_iam_instance_profile.this.name
  key_name               = aws_key_pair.this.key_name

  user_data = templatefile("${path.module}/user_data.sh.tftpl", {
    aws_region = var.aws_region
    ssm_prefix = var.ssm_param_prefix
    api_domain = var.api_domain
    ghcr_owner = var.ghcr_owner
    bots       = var.bots
  })

  root_block_device {
    volume_size = var.root_volume_size_gb
    volume_type = "gp3"
    encrypted   = true
  }

  metadata_options {
    http_tokens = "required" # IMDSv2 only
    http_endpoint = "enabled"
  }

  tags = merge(var.tags, { Name = "${var.name_prefix}-bot" })

  lifecycle {
    ignore_changes = [
      ami, # don't recreate when AMI ID drifts; rotate via explicit refresh
    ]
  }
}

resource "aws_eip" "this" {
  domain   = "vpc"
  instance = aws_instance.this.id
  tags     = merge(var.tags, { Name = "${var.name_prefix}-bot" })
}
```

- [ ] **Step 3: `infra/modules/bot_host/user_data.sh.tftpl`**

```bash
#!/usr/bin/env bash
set -euxo pipefail

dnf update -y
dnf install -y docker jq

# Docker
systemctl enable --now docker
usermod -aG docker ec2-user

# docker compose plugin
mkdir -p /usr/local/lib/docker/cli-plugins
curl -sSL "https://github.com/docker/compose/releases/latest/download/docker-compose-linux-aarch64" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose

# Caddy (Amazon Linux 2023 — install from RPM)
CADDY_VERSION="2.8.4"
curl -sSL "https://github.com/caddyserver/caddy/releases/download/v$${CADDY_VERSION}/caddy_$${CADDY_VERSION}_linux_arm64.tar.gz" | tar -xz -C /usr/local/bin caddy
chmod +x /usr/local/bin/caddy
groupadd --system caddy || true
useradd --system --gid caddy --create-home --home-dir /var/lib/caddy --shell /usr/sbin/nologin caddy || true
mkdir -p /etc/caddy /var/log/caddy

cat > /etc/caddy/Caddyfile <<CADDY
${api_domain} {
  reverse_proxy localhost:8080
  log {
    output file /var/log/caddy/access.log
  }
}
CADDY

cat > /etc/systemd/system/caddy.service <<SVC
[Unit]
Description=Caddy
After=network.target

[Service]
User=caddy
Group=caddy
ExecStart=/usr/local/bin/caddy run --config /etc/caddy/Caddyfile
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/Caddyfile
Restart=on-failure
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
SVC

mkdir -p /opt/rollton
chown ec2-user:ec2-user /opt/rollton

cat > /opt/rollton/docker-compose.yml <<'COMPOSE'
services:
%{ for bot in bots ~}
  ${bot.name}:
    image: ghcr.io/${ghcr_owner}/rollton-${bot.name}:${bot.tag}
    container_name: rollton-${bot.name}
    restart: unless-stopped
    env_file:
      - .env.${bot.name}
    ports:
      - "${bot.port}:${bot.port}"
%{ endfor ~}
COMPOSE

cat > /usr/local/bin/rollton-fetch-env.sh <<'FETCH'
#!/usr/bin/env bash
set -e
AWS_REGION="${aws_region}"
SSM_PREFIX="${ssm_prefix}"
DB_URL=$(aws ssm get-parameter --region "$AWS_REGION" --name "$SSM_PREFIX/database_url" --with-decryption --query Parameter.Value --output text)
%{ for bot in bots ~}
TOKEN=$(aws ssm get-parameter --region "$AWS_REGION" --name "$SSM_PREFIX/${bot.name}/telegram_token" --with-decryption --query Parameter.Value --output text 2>/dev/null || echo "")
cat > /opt/rollton/.env.${bot.name} <<EOF
TELEGRAM_TOKEN=$TOKEN
DATABASE_URL=$DB_URL
HTTP_PORT=${bot.port}
LOG_LEVEL=info
LOG_FORMAT=json
EOF
%{ endfor ~}
chmod 600 /opt/rollton/.env.*
chown ec2-user:ec2-user /opt/rollton/.env.*
FETCH
chmod +x /usr/local/bin/rollton-fetch-env.sh

cat > /etc/systemd/system/rollton-env.service <<SVC
[Unit]
Description=Fetch Rollton secrets from SSM into /opt/rollton/.env.*
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/rollton-fetch-env.sh
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
SVC

cat > /etc/systemd/system/rollton-compose.service <<SVC
[Unit]
Description=Rollton bots (Docker Compose)
Requires=rollton-env.service docker.service
After=rollton-env.service docker.service

[Service]
Type=oneshot
WorkingDirectory=/opt/rollton
ExecStart=/usr/bin/docker compose up -d --remove-orphans --pull always
ExecStop=/usr/bin/docker compose down
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
SVC

systemctl daemon-reload
systemctl enable --now caddy.service
systemctl enable --now rollton-env.service
systemctl enable --now rollton-compose.service
```

(Note: `$${CADDY_VERSION}` and `$$(...)` use double-dollar to escape Terraform's templatefile interpolation so bash sees a single `$`.)

- [ ] **Step 4: `infra/modules/bot_host/outputs.tf`**

```hcl
output "instance_id" {
  value = aws_instance.this.id
}

output "public_ip" {
  value = aws_eip.this.public_ip
}

output "security_group_id" {
  value = aws_security_group.this.id
}

output "iam_role_arn" {
  value = aws_iam_role.this.arn
}
```

- [ ] **Step 5: Format**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
```

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/modules/bot_host/
git commit -m "feat(infra): bot_host module — EC2, EIP, SG, IAM, Caddy + compose bootstrap"
```

---

## Task 6: `modules/secrets`

**Files:**
- Create: `infra/modules/secrets/{main.tf, variables.tf, outputs.tf}`

- [ ] **Step 1: `infra/modules/secrets/variables.tf`**

```hcl
variable "prefix" {
  description = "SSM path prefix, e.g. /rollton/prod"
  type        = string
}

variable "database_url" {
  description = "Postgres connection string"
  type        = string
  sensitive   = true
}

variable "bot_names" {
  description = "Bot names to provision telegram_token slots for"
  type        = list(string)
}

variable "tags" {
  type    = map(string)
  default = {}
}
```

- [ ] **Step 2: `infra/modules/secrets/main.tf`**

```hcl
resource "aws_ssm_parameter" "database_url" {
  name        = "${var.prefix}/database_url"
  description = "Postgres connection string for the rollton-prod RDS instance"
  type        = "SecureString"
  value       = var.database_url
  tags        = var.tags
}

# Token slots — Terraform creates the parameter with a placeholder, then ignores
# subsequent value changes. The operator runs `aws ssm put-parameter --overwrite`
# (or pastes via the console) to set the real BotFather token without Terraform fighting it back.
resource "aws_ssm_parameter" "telegram_token" {
  for_each = toset(var.bot_names)

  name        = "${var.prefix}/${each.key}/telegram_token"
  description = "Telegram bot token for ${each.key} — set manually via BotFather"
  type        = "SecureString"
  value       = "PLACEHOLDER_SET_VIA_AWS_SSM_PUT_PARAMETER"
  tags        = var.tags

  lifecycle {
    ignore_changes = [value]
  }
}
```

- [ ] **Step 3: `infra/modules/secrets/outputs.tf`**

```hcl
output "database_url_arn" {
  value = aws_ssm_parameter.database_url.arn
}

output "token_arns" {
  value = { for k, v in aws_ssm_parameter.telegram_token : k => v.arn }
}
```

- [ ] **Step 4: Format + commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/modules/secrets/
git commit -m "feat(infra): secrets module — SSM Parameter Store entries"
```

---

## Task 7: `modules/dns`

**Files:**
- Create: `infra/modules/dns/{main.tf, variables.tf, outputs.tf}`

- [ ] **Step 1: `infra/modules/dns/variables.tf`**

```hcl
variable "cloudflare_zone_id" {
  type = string
}

variable "api_subdomain" {
  description = "Subdomain for the bot API, e.g. \"api\""
  type        = string
  default     = "api"
}

variable "api_target_ip" {
  description = "EIP of the bot host"
  type        = string
}

variable "app_subdomain" {
  description = "Subdomain for the mini app, e.g. \"app\""
  type        = string
  default     = "app"
}

variable "app_target_cname" {
  description = "Cloudflare Pages CNAME target (e.g. rollton-web.pages.dev)"
  type        = string
}
```

- [ ] **Step 2: `infra/modules/dns/main.tf`**

```hcl
resource "cloudflare_record" "api" {
  zone_id = var.cloudflare_zone_id
  name    = var.api_subdomain
  type    = "A"
  content = var.api_target_ip
  proxied = false # Caddy on the EC2 handles TLS via Let's Encrypt
  ttl     = 300
  comment = "rolltonchatbot API — managed by terraform"
}

resource "cloudflare_record" "app" {
  zone_id = var.cloudflare_zone_id
  name    = var.app_subdomain
  type    = "CNAME"
  content = var.app_target_cname
  proxied = true # CF Pages serves over CF's edge
  ttl     = 1   # 1 = "auto" when proxied
  comment = "rolltonchatbot Mini App — Cloudflare Pages"
}
```

- [ ] **Step 3: `infra/modules/dns/outputs.tf`**

```hcl
output "api_fqdn" {
  value = cloudflare_record.api.hostname
}

output "app_fqdn" {
  value = cloudflare_record.app.hostname
}
```

- [ ] **Step 4: Format + commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/modules/dns/
git commit -m "feat(infra): dns module — cloudflare records for api + app"
```

---

## Task 8: `modules/github_oidc`

**Files:**
- Create: `infra/modules/github_oidc/{main.tf, variables.tf, outputs.tf}`

- [ ] **Step 1: `infra/modules/github_oidc/variables.tf`**

```hcl
variable "github_repo" {
  description = "GitHub repo slug (owner/repo)"
  type        = string
}

variable "allowed_refs" {
  description = "Git refs allowed to assume the role"
  type        = list(string)
  default     = ["refs/heads/main"]
}

variable "role_name" {
  type    = string
  default = "rollton-github-actions"
}

variable "tags" {
  type    = map(string)
  default = {}
}
```

- [ ] **Step 2: `infra/modules/github_oidc/main.tf`**

```hcl
resource "aws_iam_openid_connect_provider" "github" {
  url             = "https://token.actions.githubusercontent.com"
  client_id_list  = ["sts.amazonaws.com"]
  thumbprint_list = ["6938fd4d98bab03faadb97b34396831e3780aea1"] # GitHub's well-known OIDC thumbprint
  tags            = var.tags
}

locals {
  trust_subjects = [for ref in var.allowed_refs : "repo:${var.github_repo}:ref:${ref}"]
}

resource "aws_iam_role" "github_actions" {
  name = var.role_name

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Principal = {
        Federated = aws_iam_openid_connect_provider.github.arn
      }
      Action = "sts:AssumeRoleWithWebIdentity"
      Condition = {
        StringEquals = {
          "token.actions.githubusercontent.com:aud" = "sts.amazonaws.com"
        }
        StringLike = {
          "token.actions.githubusercontent.com:sub" = local.trust_subjects
        }
      }
    }]
  })

  tags = var.tags
}

# For the stub we attach AdministratorAccess to keep `infra.yml` plan/apply
# unblocked. Tighten to least-privilege when the resource set stabilizes.
resource "aws_iam_role_policy_attachment" "admin" {
  role       = aws_iam_role.github_actions.name
  policy_arn = "arn:aws:iam::aws:policy/AdministratorAccess"
}
```

- [ ] **Step 3: `infra/modules/github_oidc/outputs.tf`**

```hcl
output "role_arn" {
  value       = aws_iam_role.github_actions.arn
  description = "Paste into .github/workflows/infra.yml as role-to-assume"
}

output "oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.github.arn
}
```

- [ ] **Step 4: Format + commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/modules/github_oidc/
git commit -m "feat(infra): github_oidc module — OIDC provider + IAM role"
```

---

## Task 9: Wire modules in `envs/prod/main.tf`

**Files:**
- Modify: `infra/envs/prod/main.tf`, `infra/envs/prod/outputs.tf`

- [ ] **Step 1: Rewrite `infra/envs/prod/main.tf`**

```hcl
data "aws_caller_identity" "current" {}

module "network" {
  source = "../../modules/network"

  name_prefix = var.name_prefix
  azs         = local.azs
  tags        = local.common_tags
}

module "bot_host" {
  source = "../../modules/bot_host"

  name_prefix      = var.name_prefix
  aws_region       = var.aws_region
  vpc_id           = module.network.vpc_id
  subnet_id        = module.network.public_subnet_ids[0]
  instance_type    = var.bot_instance_type
  ssh_public_key   = var.ssh_public_key
  operator_ip_cidr = var.operator_ip_cidr
  ssm_param_prefix = local.ssm_prefix
  api_domain       = local.api_domain
  ghcr_owner       = var.ghcr_owner
  bots             = local.bots
  tags             = local.common_tags
}

module "database" {
  source = "../../modules/database"

  name_prefix           = var.name_prefix
  vpc_id                = module.network.vpc_id
  subnet_ids            = module.network.public_subnet_ids
  bot_security_group_id = module.bot_host.security_group_id
  instance_class        = var.db_instance_class
  allocated_storage     = var.db_allocated_storage
  tags                  = local.common_tags
}

module "secrets" {
  source = "../../modules/secrets"

  prefix       = local.ssm_prefix
  database_url = module.database.connection_string
  bot_names    = [for b in local.bots : b.name]
  tags         = local.common_tags
}

module "dns" {
  source = "../../modules/dns"

  cloudflare_zone_id = var.cloudflare_zone_id
  api_target_ip      = module.bot_host.public_ip
  app_target_cname   = var.cloudflare_pages_cname
}

module "github_oidc" {
  source = "../../modules/github_oidc"

  github_repo = var.github_repo
  tags        = local.common_tags
}
```

- [ ] **Step 2: Rewrite `infra/envs/prod/outputs.tf`**

```hcl
output "aws_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "bot_host_public_ip" {
  description = "Use this for the EC2_HOST GitHub secret"
  value       = module.bot_host.public_ip
}

output "bot_host_instance_id" {
  value = module.bot_host.instance_id
}

output "db_endpoint" {
  value = module.database.endpoint
}

output "db_connection_string" {
  value     = module.database.connection_string
  sensitive = true
}

output "api_fqdn" {
  value = module.dns.api_fqdn
}

output "app_fqdn" {
  value = module.dns.app_fqdn
}

output "github_actions_role_arn" {
  description = "Paste into .github/workflows/infra.yml as role-to-assume"
  value       = module.github_oidc.role_arn
}
```

- [ ] **Step 3: Format**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -recursive
```

- [ ] **Step 4: Validate (modules wired correctly)**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra/envs/prod
terraform init -backend=false -input=false
terraform validate
```
Expected: `Success! The configuration is valid.`

- [ ] **Step 5: Clean up provider download** (keep `.terraform.lock.hcl`)

```bash
rm -rf .terraform
```

- [ ] **Step 6: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/envs/prod/main.tf infra/envs/prod/outputs.tf infra/envs/prod/.terraform.lock.hcl
git commit -m "feat(infra): compose all six modules in envs/prod"
```

---

## Task 10: GitHub Actions workflows

**Files:**
- Create: `.github/workflows/{web.yml, bot.yml, infra.yml}`

- [ ] **Step 1: `.github/workflows/web.yml`**

```yaml
name: web

on:
  pull_request:
    paths: ['web/**']
  push:
    branches: [main]
    paths: ['web/**']

jobs:
  ci:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: web
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: 'npm'
          cache-dependency-path: web/package-lock.json

      - run: npm ci
      - run: npm run lint
      - run: npm run typecheck
      - run: npm run test
      - run: npm run build
```

- [ ] **Step 2: `.github/workflows/bot.yml`**

```yaml
name: bot

on:
  pull_request:
    paths: ['bot/**']
  push:
    branches: [main]
    paths: ['bot/**']

permissions:
  contents: read
  packages: write

jobs:
  test:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: bot
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache-dependency-path: bot/go.sum

      - run: go vet ./...
      - run: go test -race -short ./...

  build-and-deploy:
    needs: test
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: docker/setup-qemu-action@v3
      - uses: docker/setup-buildx-action@v3

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build & push rolltonchatbot
        uses: docker/build-push-action@v6
        with:
          context: ./bot
          file: ./bot/Dockerfile
          platforms: linux/arm64
          build-args: BOT=rolltonchatbot
          push: true
          tags: |
            ghcr.io/${{ github.repository_owner }}/rollton-rolltonchatbot:latest
            ghcr.io/${{ github.repository_owner }}/rollton-rolltonchatbot:${{ github.sha }}

      - name: Build & push admin
        uses: docker/build-push-action@v6
        with:
          context: ./bot
          file: ./bot/Dockerfile
          platforms: linux/arm64
          build-args: BOT=admin
          push: true
          tags: |
            ghcr.io/${{ github.repository_owner }}/rollton-admin:latest
            ghcr.io/${{ github.repository_owner }}/rollton-admin:${{ github.sha }}

      - name: Deploy to EC2
        uses: appleboy/ssh-action@v1
        with:
          host: ${{ secrets.EC2_HOST }}
          username: ec2-user
          key: ${{ secrets.EC2_SSH_KEY }}
          script: |
            set -e
            cd /opt/rollton
            docker compose pull
            docker compose up -d --remove-orphans
            docker image prune -f
```

- [ ] **Step 3: `.github/workflows/infra.yml`**

```yaml
name: infra

on:
  pull_request:
    paths: ['infra/**']
  push:
    branches: [main]
    paths: ['infra/**']

permissions:
  contents: read
  id-token: write
  pull-requests: write

env:
  TF_INPUT: '0'
  AWS_REGION: eu-central-1
  TF_VAR_cloudflare_api_token: ${{ secrets.CLOUDFLARE_API_TOKEN }}

jobs:
  plan:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: infra/envs/prod
    steps:
      - uses: actions/checkout@v4

      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: '1.10.5'
          terraform_wrapper: false

      - name: fmt-check
        run: terraform fmt -check -recursive
        working-directory: infra

      - run: terraform init
      - run: terraform validate

      - name: plan
        id: plan
        run: terraform plan -no-color -out=tfplan
        continue-on-error: true

      - name: Comment plan on PR
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        env:
          PLAN_OUT: ${{ steps.plan.outputs.stdout }}
        with:
          script: |
            const body = `### Terraform Plan\n\`\`\`\n${process.env.PLAN_OUT || '(empty)'}\n\`\`\``
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body
            })

      - name: Upload plan artifact
        if: github.ref == 'refs/heads/main'
        uses: actions/upload-artifact@v4
        with:
          name: tfplan
          path: infra/envs/prod/tfplan

  apply:
    needs: plan
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    environment: production
    defaults:
      run:
        working-directory: infra/envs/prod
    steps:
      - uses: actions/checkout@v4

      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: ${{ secrets.AWS_OIDC_ROLE_ARN }}
          aws-region: ${{ env.AWS_REGION }}

      - uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: '1.10.5'
          terraform_wrapper: false

      - uses: actions/download-artifact@v4
        with:
          name: tfplan
          path: infra/envs/prod

      - run: terraform init
      - run: terraform apply -auto-approve tfplan
```

- [ ] **Step 4: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add .github/workflows/
git commit -m "ci: add web, bot, infra workflows"
```

---

## Task 11: Local verification

- [ ] **Step 1: Format check (no diff allowed)**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
terraform fmt -check -recursive
```
Expected: exits 0, no output.

- [ ] **Step 2: Validate the bootstrap module**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra/bootstrap
terraform init -backend=false -input=false
terraform validate
```
Expected: `Success! The configuration is valid.`

- [ ] **Step 3: Validate envs/prod**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra/envs/prod
terraform init -backend=false -input=false
terraform validate
```
Expected: `Success! The configuration is valid.`

- [ ] **Step 4: Clean leftover `.terraform` dirs**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton/infra
find . -type d -name '.terraform' -exec rm -rf {} +
```

- [ ] **Step 5: YAML lint the workflows (smoke check)**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
python3 -c "import yaml,sys; [yaml.safe_load(open(f)) for f in ['.github/workflows/web.yml','.github/workflows/bot.yml','.github/workflows/infra.yml']]; print('OK')"
```
Expected: `OK`. (If the host lacks `pyyaml`, skip; GitHub will validate on push.)

- [ ] **Step 6: Confirm clean tree**

```bash
git status
```
Expected: `nothing to commit, working tree clean` (apart from the deleted `.terraform/` dirs which were never tracked).

---

## Task 12: Operator runbook

**Files:**
- Modify: `infra/README.md` (add the runbook section)

- [ ] **Step 1: Append the runbook to `infra/README.md`**

Add at the end of `infra/README.md`:

```markdown

## End-to-end first-time setup runbook

The order matters — some steps depend on outputs of earlier steps.

### 0. Prerequisites
- AWS account, admin IAM user with access keys *configured locally* (for bootstrap + first apply)
- Cloudflare account, domain `rollton.com` added to it with nameservers set at the registrar
- Cloudflare API token with `Zone:DNS:Edit` on the zone
- SSH key pair on your machine (`ssh-keygen -t ed25519 -C operator@rollton`)
- A GitHub Personal Access Token or use the workflow's `GITHUB_TOKEN` (the workflow handles GHCR auth automatically)
- The `rolltonchatbot/admin` Docker images pushed to GHCR via `bot.yml` workflow at least once (or build & push manually)

### 1. Bootstrap state bucket
```
cd infra/bootstrap
terraform init
terraform apply -var "state_bucket_name=rollton-tfstate-<YOUR_AWS_ACCOUNT_ID>"
```
Copy the bucket name into `infra/envs/prod/backend.tf` (replace the `rollton-tfstate` placeholder).

### 2. First apply of the main config
```
cd infra/envs/prod
cp terraform.tfvars.example terraform.tfvars   # fill values
terraform init                                 # uses S3 backend now
terraform apply
```

### 3. Populate manually-set SSM parameters
```
aws ssm put-parameter --region eu-central-1 --overwrite \
  --name /rollton/prod/rolltonchatbot/telegram_token \
  --type SecureString \
  --value "8833471135:..."

aws ssm put-parameter --region eu-central-1 --overwrite \
  --name /rollton/prod/admin/telegram_token \
  --type SecureString \
  --value "..."
```

Then SSH in and trigger a re-fetch:
```
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
```

- [ ] **Step 2: Commit**

```bash
cd /Users/konstantinkotau/Desktop/projects.com/rollton
git add infra/README.md
git commit -m "docs(infra): operator runbook for first-time setup"
```

---

## Out-of-scope (intentional, deferred)

- Staging environment (clone `envs/prod` → `envs/staging` when needed)
- Multi-AZ RDS, read replicas
- CloudWatch dashboards & alarms
- Cost budgets / alerts (set manually in AWS Console for now)
- ECR (using GHCR)
- VPC endpoints for SSM / S3 (would save ~$0 at our scale)
- IAM least-privilege for the GHA role (currently AdministratorAccess; tighten when stable)
- Backup automation beyond RDS defaults
- DR / cross-region replication
- WAF on Caddy
- Bastion host (SSH direct to EC2 via key + IP allow-list is fine)
- Terraform-managed Cloudflare Pages project (using dashboard Git integration)
- Public vs private GHCR — defaulting to public packages so EC2 pulls work without auth. If preferred private, add a personal access token in SSM and have `user_data` `docker login ghcr.io` before pulling.

## Open items resolved at execution time

- AWS account ID, Cloudflare account/zone IDs, operator IP, SSH key, GHCR image tags.
- Whether `rollton.com` is already at Cloudflare (assumed; if it's at Route53 instead, swap the `dns` module).
- Caddy version (`2.8.4` pinned in user_data) may want refreshing periodically.
- AWS Linux 2023 AMI ID is selected dynamically via `data.aws_ami` — fine, but `ignore_changes = [ami]` is set on the instance to avoid surprise re-creates. Refresh AMI explicitly via `terraform taint` when desired.
- The bot.yml workflow currently runs migrations as part of the SSH `script:` block — the SSH script needs the bot image to have `goose` installed. The current Dockerfile only ships the Go binary. **Action at execution time:** either (a) add `goose` to the Dockerfile's runtime stage, or (b) move migrations to a one-shot init container that runs before bots start. The plan's task list does NOT include this Dockerfile change — fix during execution.
