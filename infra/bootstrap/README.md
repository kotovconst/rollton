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
