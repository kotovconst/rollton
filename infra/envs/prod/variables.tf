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
