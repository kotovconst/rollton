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
