variable "cloudflare_api_token" {
  type        = string
  description = "Cloudflare API Token with Zone.DNS and Zone.Zone settings edit permissions"
  sensitive   = true
}

variable "domain_name" {
  type        = string
  description = "Root domain name managed in Cloudflare"
  default     = "vid2vec.example.com"
}

variable "record_type" {
  type        = string
  description = "DNS record type pointing to origin (CNAME for AWS ALB hostname, A for GCP Load Balancer IP)"
  default     = "CNAME"

  validation {
    condition     = contains(["CNAME", "A"], var.record_type)
    error_message = "record_type must be either 'CNAME' or 'A'."
  }
}

variable "target_address" {
  type        = string
  description = "Target load balancer hostname (AWS ALB) or IPv4 address (GCP Ingress)"
}

variable "api_subdomain" {
  type        = string
  description = "Subdomain prefix for the API service"
  default     = "api"
}

variable "proxied" {
  type        = bool
  description = "Enable Cloudflare orange-cloud proxy for DDoS and CDN"
  default     = true
}

variable "create_root_record" {
  type        = bool
  description = "Create CNAME flattening record on root domain pointing to the target"
  default     = false
}

variable "ssl_mode" {
  type        = string
  description = "SSL encryption mode between Cloudflare and origin"
  default     = "strict"

  validation {
    condition     = contains(["strict", "full", "flexible", "off"], var.ssl_mode)
    error_message = "ssl_mode must be one of: strict, full, flexible, off."
  }
}

variable "acm_validation_records" {
  type        = map(string)
  description = "Map of ACM validation record names to CNAME values for AWS origin TLS"
  default     = {}
}
