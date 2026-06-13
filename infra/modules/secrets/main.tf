resource "aws_ssm_parameter" "database_url" {
  name        = "${var.prefix}/database_url"
  description = "Postgres connection string for the rollton-prod RDS instance"
  type        = "SecureString"
  value       = var.database_url
  tags        = var.tags
}

# Token slots — Terraform creates the parameter with a placeholder, then ignores
# subsequent value changes. The operator runs `aws ssm put-parameter --overwrite`
# (or pastes via the console) to set the real BotFather token without Terraform fighting it back.
resource "aws_ssm_parameter" "telegram_token" {
  for_each = toset(var.bot_names)

  name        = "${var.prefix}/${each.key}/telegram_token"
  description = "Telegram bot token for ${each.key} — set manually via BotFather"
  type        = "SecureString"
  value       = "PLACEHOLDER_SET_VIA_AWS_SSM_PUT_PARAMETER"
  tags        = var.tags

  lifecycle {
    ignore_changes = [value]
  }
}
