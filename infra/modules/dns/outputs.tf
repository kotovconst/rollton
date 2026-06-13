output "api_fqdn" {
  value = cloudflare_record.api.hostname
}

output "app_fqdn" {
  value = cloudflare_record.app.hostname
}
