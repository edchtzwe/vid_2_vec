terraform {
  required_version = ">= 1.5.0"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.0"
    }
  }
}

provider "cloudflare" {
  api_token = var.cloudflare_api_token
}

data "cloudflare_zone" "primary" {
  name = var.domain_name
}

resource "cloudflare_record" "api" {
  zone_id = data.cloudflare_zone.primary.id
  name    = var.api_subdomain
  content = var.target_address
  type    = var.record_type
  proxied = var.proxied
  ttl     = var.proxied ? 1 : 300
}

resource "cloudflare_record" "root" {
  count   = var.create_root_record ? 1 : 0
  zone_id = data.cloudflare_zone.primary.id
  name    = "@"
  content = var.target_address
  type    = var.record_type
  proxied = var.proxied
  ttl     = var.proxied ? 1 : 300
}

resource "cloudflare_record" "acm_validation" {
  for_each = var.acm_validation_records

  zone_id = data.cloudflare_zone.primary.id
  name    = each.key
  content = each.value
  type    = "CNAME"
  proxied = false
  ttl     = 60
}

resource "cloudflare_zone_settings_override" "primary" {
  zone_id = data.cloudflare_zone.primary.id

  settings {
    ssl                      = var.ssl_mode
    always_use_https         = "on"
    min_tls_version          = "1.2"
    http3                    = "on"
    zero_rtt                 = "on"
    brotli                   = "on"
    automatic_https_rewrites = "on"
  }
}
