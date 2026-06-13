output "database_url_arn" {
  value = aws_ssm_parameter.database_url.arn
}

output "token_arns" {
  value = { for k, v in aws_ssm_parameter.telegram_token : k => v.arn }
}
