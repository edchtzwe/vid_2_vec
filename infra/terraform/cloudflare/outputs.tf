output "zone_id" {
  description = "Cloudflare Zone ID"
  value       = data.cloudflare_zone.primary.id
}

output "domain_name" {
  description = "Managed domain name"
  value       = var.domain_name
}

output "api_fqdn" {
  description = "Fully qualified domain name for the API service"
  value       = "${var.api_subdomain}.${var.domain_name}"
}

output "nameservers" {
  description = "Assigned Cloudflare nameservers"
  value       = data.cloudflare_zone.primary.name_servers
}

output "ssl_mode" {
  description = "Configured SSL mode"
  value       = var.ssl_mode
}

output "proxied" {
  description = "Status of Cloudflare proxying"
  value       = var.proxied
}
