locals {
  common_tags = {
    Project     = "rollton"
    Environment = "prod"
    ManagedBy   = "terraform"
  }

  api_domain = "api.${var.domain_root}"
  app_domain = "app.${var.domain_root}"

  ssm_prefix = "/rollton/prod"

  # Bots running on the host. Adding one is one more entry here + a new SSM token.
  bots = [
    { name = "rolltonchatbot", port = 8080, tag = "latest" },
    { name = "admin", port = 8081, tag = "latest" },
  ]

  azs = [
    "${var.aws_region}a",
    "${var.aws_region}b",
  ]
}
