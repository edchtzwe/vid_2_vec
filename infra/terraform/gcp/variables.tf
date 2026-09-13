variable "project_id" {
  type        = string
  description = "GCP project ID. Find it with: gcloud config get-value project"
}

variable "region" {
  type        = string
  description = "GCP Region"
  default     = "us-central1"
}

variable "zones" {
  type        = list(string)
  description = "GCP Zones for multi-zone node pool deployment"
  default     = ["us-central1-a", "us-central1-b"]
}

variable "network_name" {
  type        = string
  description = "Name of the VPC network"
  default     = "vid2vec-gcp-vpc"
}

variable "subnet_cidr" {
  type        = string
  description = "CIDR block for the primary subnet"
  default     = "10.10.0.0/20"
}

variable "pods_cidr" {
  type        = string
  description = "Secondary CIDR for GKE Pods"
  default     = "10.20.0.0/16"
}

variable "services_cidr" {
  type        = string
  description = "Secondary CIDR for GKE Services"
  default     = "10.30.0.0/20"
}

variable "cluster_name" {
  type        = string
  description = "Name of the GKE cluster"
  default     = "vid2vec-gke-cluster"
}

variable "machine_type" {
  type        = string
  description = "Compute Engine machine type for worker nodes"
  default     = "e2-standard-2"
}

variable "min_node_count" {
  type        = number
  description = "Minimum node count per zone for failover"
  default     = 1
}

variable "max_node_count" {
  type        = number
  description = "Maximum node count per zone"
  default     = 3
}

variable "crossplane_namespace" {
  type        = string
  description = "Kubernetes namespace for Crossplane"
  default     = "crossplane-system"
}

variable "artifact_registry_repository_id" {
  type        = string
  description = "Artifact Registry Docker repository ID for the application container image"
  default     = "vid2vec"
}

variable "crossplane_chart_version" {
  type        = string
  description = "Helm chart version for Crossplane"
  default     = "1.16.0"
}

variable "domain_name" {
  type        = string
  description = "Primary domain name for Cloud DNS (e.g. vid2vec.example.com)"
  default     = "vid2vec.example.com"
}

variable "enable_cloud_dns" {
  type        = bool
  description = "Enable Cloud DNS managed zone (set false if using Cloudflare)"
  default     = true
}
