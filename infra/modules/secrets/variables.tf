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
