resource "cloudflare_record" "api" {
  zone_id = var.cloudflare_zone_id
  name    = var.api_subdomain
  type    = "A"
  content = var.api_target_ip
  proxied = false # Caddy on the EC2 handles TLS via Let's Encrypt
  ttl     = 300
  comment = "rolltonchatbot API — managed by terraform"
}

resource "cloudflare_record" "app" {
  zone_id = var.cloudflare_zone_id
  name    = var.app_subdomain
  type    = "CNAME"
  content = var.app_target_cname
  proxied = true # CF Pages serves over CF's edge
  ttl     = 1    # 1 = "auto" when proxied
  comment = "rolltonchatbot Mini App — Cloudflare Pages"
}
