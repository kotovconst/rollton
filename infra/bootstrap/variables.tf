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
