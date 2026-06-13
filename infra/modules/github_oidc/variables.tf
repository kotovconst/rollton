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
