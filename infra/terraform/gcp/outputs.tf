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
