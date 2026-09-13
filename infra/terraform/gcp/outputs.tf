output "cluster_name" {
  description = "GKE Cluster Name"
  value       = google_container_cluster.primary.name
}

output "cluster_endpoint" {
  description = "GKE Cluster API Endpoint"
  value       = google_container_cluster.primary.endpoint
}

output "workload_identity_pool" {
  description = "GKE Workload Identity Pool"
  value       = "${var.project_id}.svc.id.goog"
}

output "crossplane_service_account_email" {
  description = "GCP Service Account Email for Crossplane Provider-GCP"
  value       = google_service_account.crossplane_gcp.email
}

output "artifact_registry_repository" {
  description = "Artifact Registry repository path for the application container image"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.app.repository_id}"
}

output "network_name" {
  description = "VPC Network Name"
  value       = google_compute_network.vpc.name
}

output "dns_name_servers" {
  description = "Cloud DNS name servers to set in GoDaddy"
  value       = google_dns_managed_zone.primary.name_servers
}
