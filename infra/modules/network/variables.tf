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
