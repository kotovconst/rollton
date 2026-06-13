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
