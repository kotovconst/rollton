# ─────────────────────────────────────────────────────────────────────────────
# Terraform STATE backend — NOT the bot's API backend.
#
# This block tells Terraform where to store its own bookkeeping file
# (terraform.tfstate) that maps Terraform resources to real AWS resource IDs.
# Without this, two engineers (or a CI job) running terraform apply would not
# share a source of truth, and rerunning Terraform from a different machine
# would not know about previously created resources.
#
# The bot's API backend (the Go service) runs on EC2 — see modules/bot_host.
# This S3 bucket holds ONE small JSON file. Cost is fractions of a cent/month.
#
# The bucket is created by infra/bootstrap/ (one-shot) before this config is
# initialized. Update the `bucket` value below if your bootstrap created a
# different name.
# ─────────────────────────────────────────────────────────────────────────────
terraform {
  backend "s3" {
    bucket       = "rollton-tfstate"
    key          = "envs/prod/terraform.tfstate"
    region       = "eu-central-1"
    encrypt      = true
    use_lockfile = true
  }
}
