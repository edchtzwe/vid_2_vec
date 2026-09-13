terraform {
  required_version = ">= 1.5.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    helm = {
      source  = "hashicorp/helm"
      version = "~> 2.12"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

# ─────────────────────────────────────────────────────────────────────────────
# Networking (VPC, Subnet with Secondary Ranges for GKE Pods & Services)
# ─────────────────────────────────────────────────────────────────────────────

resource "google_compute_network" "vpc" {
  name                    = var.network_name
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "subnet" {
  name          = "${var.network_name}-subnet"
  ip_cidr_range = var.subnet_cidr
  region        = var.region
  network       = google_compute_network.vpc.id

  secondary_ip_range {
    range_name    = "gke-pods"
    ip_cidr_range = var.pods_cidr
  }

  secondary_ip_range {
    range_name    = "gke-services"
    ip_cidr_range = var.services_cidr
  }
}

resource "google_compute_router" "router" {
  name    = "${var.network_name}-router"
  region  = var.region
  network = google_compute_network.vpc.id
}

resource "google_compute_router_nat" "nat" {
  name                               = "${var.network_name}-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# ─────────────────────────────────────────────────────────────────────────────
# GKE Cluster with Workload Identity
# ─────────────────────────────────────────────────────────────────────────────

resource "google_service_account" "gke_nodes" {
  account_id   = "${var.cluster_name}-node-sa"
  display_name = "GKE Node Service Account"
}

resource "google_project_iam_member" "node_roles" {
  for_each = toset([
    "roles/logging.logWriter",
    "roles/monitoring.metricWriter",
    "roles/monitoring.viewer",
    "roles/stackdriver.resourceMetadata.writer"
  ])
  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_container_cluster" "primary" {
  name     = var.cluster_name
  location = var.region

  node_locations = var.zones

  remove_default_node_pool = true
  initial_node_count       = 1

  network    = google_compute_network.vpc.name
  subnetwork = google_compute_subnetwork.subnet.name

  ip_allocation_policy {
    cluster_secondary_range_name  = "gke-pods"
    services_secondary_range_name = "gke-services"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false
    master_ipv4_cidr_block  = "172.16.0.0/28"
  }

  deletion_protection = false
}

# ─────────────────────────────────────────────────────────────────────────────
# GKE Node Pool (2+ Nodes across AZs for Failover)
# ─────────────────────────────────────────────────────────────────────────────

resource "google_container_node_pool" "primary_nodes" {
  name     = "${var.cluster_name}-node-pool"
  location = var.region
  cluster  = google_container_cluster.primary.name

  node_locations = var.zones

  autoscaling {
    min_node_count = var.min_node_count
    max_node_count = var.max_node_count
  }

  node_config {
    machine_type    = var.machine_type
    service_account = google_service_account.gke_nodes.email
    oauth_scopes    = ["https://www.googleapis.com/auth/cloud-platform"]

    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    labels = {
      role        = "worker"
      environment = "production"
    }
  }
}

# ─────────────────────────────────────────────────────────────────────────────
# Crossplane GCP Service Account & Workload Identity Binding
# ─────────────────────────────────────────────────────────────────────────────

resource "google_service_account" "crossplane_gcp" {
  account_id   = "crossplane-provider-gcp"
  display_name = "Crossplane Provider GCP Service Account"
}

# Roles required for Crossplane to manage Cloud SQL, IAM, and Networking
resource "google_project_iam_member" "crossplane_roles" {
  for_each = toset([
    "roles/cloudsql.admin",
    "roles/iam.serviceAccountAdmin",
    "roles/resourcemanager.projectIamAdmin",
    "roles/redis.admin",
    "roles/compute.networkAdmin"
  ])
  project = var.project_id
  role    = each.key
  member  = "serviceAccount:${google_service_account.crossplane_gcp.email}"
}

# Bind Kubernetes Crossplane ServiceAccount to GCP ServiceAccount
resource "google_service_account_iam_member" "crossplane_workload_identity" {
  service_account_id = google_service_account.crossplane_gcp.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "serviceAccount:${var.project_id}.svc.id.goog[${var.crossplane_namespace}/provider-gcp]"
}

# ─────────────────────────────────────────────────────────────────────────────
# Crossplane Helm Installation
# ─────────────────────────────────────────────────────────────────────────────

data "google_client_config" "default" {}

provider "helm" {
  kubernetes {
    host                   = "https://${google_container_cluster.primary.endpoint}"
    token                  = data.google_client_config.default.access_token
    cluster_ca_certificate = base64decode(google_container_cluster.primary.master_auth[0].cluster_ca_certificate)
  }
}

resource "google_artifact_registry_repository" "app" {
  location      = var.region
  repository_id = var.artifact_registry_repository_id
  description   = "Container image repository for the application"
  format        = "DOCKER"
}

resource "helm_release" "crossplane" {
  name             = "crossplane"
  repository       = "https://charts.crossplane.io/stable"
  chart            = "crossplane"
  version          = var.crossplane_chart_version
  namespace        = var.crossplane_namespace
  create_namespace = true

  set {
    name  = "args"
    value = "{\"--enable-environment-configs\"}"
  }

  depends_on = [google_container_node_pool.primary_nodes]
}

# ─────────────────────────────────────────────────────────────────────────────
# Metrics Server (Required for HPA CPU/Memory metrics)
# ─────────────────────────────────────────────────────────────────────────────

resource "helm_release" "metrics_server" {
  name             = "metrics-server"
  repository       = "https://kubernetes-sigs.github.io/metrics-server/"
  chart            = "metrics-server"
  version          = "3.12.1"
  namespace        = "kube-system"
  create_namespace = false

  set {
    name  = "args"
    value = "{--kubelet-insecure-tls}"
  }

  depends_on = [helm_release.crossplane]
}

# ─────────────────────────────────────────────────────────────────────────────
# Cloud DNS Managed Zone
# ─────────────────────────────────────────────────────────────────────────────

resource "google_dns_managed_zone" "primary" {
  name        = replace(replace(var.domain_name, ".", "-"), "_", "-")
  dns_name    = "${var.domain_name}."
  description = "Managed DNS zone for ${var.domain_name}"

  visibility = "public"
}
