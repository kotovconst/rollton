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
