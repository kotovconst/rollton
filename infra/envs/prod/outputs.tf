output "aws_account_id" {
  value = data.aws_caller_identity.current.account_id
}

output "bot_host_public_ip" {
  description = "Use this for the EC2_HOST GitHub secret"
  value       = module.bot_host.public_ip
}

output "bot_host_instance_id" {
  value = module.bot_host.instance_id
}

output "db_endpoint" {
  value = module.database.endpoint
}

output "db_connection_string" {
  value     = module.database.connection_string
  sensitive = true
}

output "api_fqdn" {
  value = module.dns.api_fqdn
}

output "app_fqdn" {
  value = module.dns.app_fqdn
}

output "github_actions_role_arn" {
  description = "Paste into .github/workflows/infra.yml as role-to-assume"
  value       = module.github_oidc.role_arn
}
