output "role_arn" {
  value       = aws_iam_role.github_actions.arn
  description = "Paste into .github/workflows/infra.yml as role-to-assume"
}

output "oidc_provider_arn" {
  value = aws_iam_openid_connect_provider.github.arn
}
