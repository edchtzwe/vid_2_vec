# Infrastructure Architecture: Terraform & Crossplane

## 1. Architectural Overview & Mental Model

### Kubernetes Core Concepts
- **Pod vs. Container vs. Node**:
  - **Pod**: The smallest deployable computing unit in Kubernetes. It encapsulates one or more closely related containers that share the same network namespace (IP address and port space) and shared storage volumes.
  - **Container**: The encapsulated application process runtime (e.g. Docker/containerd).
  - **Node**: The worker machine (virtual machine or bare-metal host) running the Kubelet agent and container runtime. Multiple pods execute across nodes.
- **Cluster**: An aggregated pool of worker nodes managed centrally by the Kubernetes Control Plane (API server, scheduler, controller manager, and etcd), functioning as a unified compute fabric.

### Terraform vs. Crossplane
- **Terraform (Foundational Layer / Day 0-1)**:
  - Executes as a point-in-time state tool via CI/CD pipelines (`plan` -> `apply`).
  - Best suited for core foundational infrastructure that changes infrequently: VPCs, subnets, routing tables, managed Kubernetes control planes (EKS/GKE), base IAM OIDC providers, DNS hosted zones (Route53/Cloud DNS), and foundational Helm controllers (AWS Load Balancer Controller, Cluster Autoscaler, Metrics Server).
  - *Limitation*: Terraform does not actively monitor runtime drift continuously. Drift between cloud resources and state files remains undetected until the next manual or pipeline execution.
- **Crossplane (Application Infrastructure / Day 2 Continuous Reconciliation)**:
  - Runs inside Kubernetes as an active control plane controller.
  - Extends the Kubernetes API with Custom Resource Definitions (CRDs) for cloud resources (e.g. AWS RDS, GCP Cloud SQL, IAM roles/policies).
  - Continuous active reconciliation loop: Periodically queries cloud provider APIs (AWS/GCP) against the declared Kubernetes Custom Resources.
  - *Self-Healing & Drift Correction*: If a cloud setting or IAM role is accidentally modified in the cloud console, Crossplane detects the divergence and automatically reverts it back to the declared YAML specification without human intervention.

---

## 2. Infrastructure Artifacts

| File Path | Action | Summary of Changes |
| :--- | :--- | :--- |
| `infra/README.md` | Create / Sync | Architecture guide: Terraform vs Crossplane breakdown, K8s mental model, and artifact inventory table. |
| `infra/terraform/aws/main.tf` | Create / Hardened | Root AWS module: VPC, EKS cluster (2 nodes min), node autoscaling tags, OIDC provider, Crossplane, Metrics Server, AWS Load Balancer Controller, Cluster Autoscaler, Route53 zone, and ACM cert validation. |
| `infra/terraform/aws/variables.tf` | Create / Updated | AWS region, VPC CIDR, cluster name, node instance types, `domain_name`, placeholder tags. |
| `infra/terraform/aws/outputs.tf` | Create / Updated | EKS cluster endpoint, OIDC ARN, Crossplane IRSA role ARN, `route53_name_servers`, `route53_zone_id`, `acm_certificate_arn`. |
| `infra/terraform/gcp/main.tf` | Create / Hardened | Root GCP module: VPC, GKE cluster with native node autoscaling, Workload Identity, Crossplane, Metrics Server, and Cloud DNS managed zone. |
| `infra/terraform/gcp/variables.tf` | Create / Updated | GCP project ID, region, VPC subnets, machine types, `domain_name`. |
| `infra/terraform/gcp/outputs.tf` | Create / Updated | GKE cluster endpoint, Workload Identity pool, Crossplane SA email, `dns_name_servers`. |
| `infra/crossplane/aws/provider-aws.yaml` | Create | Crossplane AWS Provider & ControllerConfig with IRSA authentication. |
| `infra/crossplane/aws/rds-postgres.yaml` | Create / Hardened | Crossplane managed RDS PostgreSQL instance (`db.t4g.small`, 7-day backup retention, final snapshot) with custom `ParameterGroup` tuned for pgvector. |
| `infra/crossplane/aws/iam.yaml` | Create | Crossplane managed IAM roles/policies (Machine: admin, worker, readonly; Humans: John, James, Dave). |
| `infra/crossplane/gcp/provider-gcp.yaml` | Create | Crossplane GCP Provider & ControllerConfig with Workload Identity authentication. |
| `infra/crossplane/gcp/cloudsql-postgres.yaml` | Create / Hardened | Crossplane managed Cloud SQL PostgreSQL instance (`db-custom-1-3840`, point-in-time recovery enabled) with databaseFlags tuned for pgvector. |
| `infra/crossplane/gcp/iam.yaml` | Create | Crossplane managed GCP Service Accounts & IAM bindings (Machine: admin, worker, readonly; Humans: John, James, Dave). |
| `infra/k8s/platform/redis.yaml` | Create / Hardened | Redis standalone deployment, 10Gi PersistentVolumeClaim (`redis-data-pvc`) for queue durability, and Service for worker queue bus. |
| `infra/k8s/platform/migration.yaml` | Create | DB schema migration Job executed before application rollouts. |
| `infra/k8s/platform/db-credentials.yaml` | Create | Dynamic database connection secret generated from Crossplane outputs. |
| `infra/k8s/platform/kustomization.yaml` | Create | Platform overlay Kustomize file wiring Redis, credentials, and migration. |
| `infra/k8s/app/workers.yaml` | Create | 4 worker deployments (`uploader`, `state`, `analyzer`, `embedder`) with queue bindings & anti-affinity across 2 nodes. |
| `infra/k8s/app/api.yaml` | Create | API service deployment and ClusterIP service with health probes. |
| `infra/k8s/app/hpa.yaml` | Create | HorizontalPodAutoscalers for API and 4 worker deployments (CPU target 70%). |
| `infra/k8s/app/ingress.yaml` | Create | Ingress manifest with AWS ALB annotations, HTTP-to-HTTPS redirect, TLS termination, and drop-in tokens. |
| `infra/k8s/app/kustomization.yaml` | Create / Updated | Application overlay Kustomize file wiring workloads, HPA, and Ingress. |
| `infra/k8s/kustomization.yaml` | Create | Root Kustomize combining platform and app overlays. |
| `.github/workflows/deploy.yaml` | Create | Unified manual `workflow_dispatch` pipeline executing combined AWS/GCP deployments. |
| `.github/workflows/deploy-aws.yaml` | Create | Dedicated manual `workflow_dispatch` pipeline specialized for AWS infrastructure and workload deployments. |
| `.github/workflows/deploy-gcp.yaml` | Create | Dedicated manual `workflow_dispatch` pipeline specialized for GCP infrastructure and workload deployments. |

---

## 3. High Availability & Worker Architecture

- **Failover**: Both AWS EKS and GCP GKE node groups are provisioned with a minimum count of 2 nodes spread across multiple Availability Zones / regions.
- **Worker Queues**: The 4 distinct background workers (`worker-uploader`, `worker-state`, `worker-analyzer`, `worker-embedder`) operate as separate Kubernetes Deployments (2 replicas each, 8 pods total) connecting to a shared Redis instance via Asynq. Hard `requiredDuringSchedulingIgnoredDuringExecution` anti-affinity ensures exactly 1 replica of each worker per node.
- **Queue Durability (Zero Data Loss)**: Redis attaches a 10Gi `PersistentVolumeClaim` (`redis-data-pvc`) using standard cloud block storage (gp3 on AWS, pd-ssd on GCP). Any pod restart or node reschedule retains active queue jobs and Asynq task payloads.
- **Health Probes**:
  - **API** (`/health-check`): `startupProbe` (5s delay, 10 retries), `livenessProbe` (15s interval, 3 failures), `readinessProbe` (10s interval, 3 failures). Removes from Service endpoint on failure.
  - **Workers** (exec `kill -0 1`): `startupProbe` (5s delay, 10 retries), `livenessProbe` (15s interval, 3 failures). No `readinessProbe` — workers consume from Redis queues, not K8s Services.
  - **Redis** (exec `redis-cli ping`): All three probes. Removes from Service during RDB/AOF load.
- **IAM Structure (Crossplane)**:
  - **Machine Roles** (for workloads & service accounts):
    1. **admin**: Full infrastructure authority — automation pipelines, break-glass workloads.
    2. **worker**: Day-to-day service read/write (EKS/GKE, RDS/CloudSQL, S3/Storage, logs) — no IAM mutation.
    3. **readonly**: Zero mutation — monitoring, health-checks, metrics scrapers.
  - **Human Accounts** (IAM Users / GCP Members, mapped to roles):
    - **John** → admin: Full infrastructure and platform admin.
    - **James** → worker: Services read/write, no IAM admin.
    - **Dave** → readonly: View and inspect only, zero changes.
- **Full-Chain Scaling Architecture (HPA + Node Autoscalers)**:
  - **Workers & API (Pod Tier)**: CPU-based HPA (target 70% utilization). Min 2 replicas, max 4. Scale-down stabilization window of 300s prevents flapping.
  - **Node Tier (Hardware VM Scaling)**:
    - **AWS EKS**: Cluster Autoscaler is deployed via Helm and IRSA in Terraform. ASG node group tags (`k8s.io/cluster-autoscaler/enabled`) trigger automatic node provisioning when HPA pods sit in `Pending`.
    - **GCP GKE**: Managed native node pool autoscaling (`autoscaling` block in `google_container_node_pool`) scales nodes from 1 to 3 per zone.
  - **Redis**: No HPA. Redis is a singleton queue bus. Scaling Redis horizontally splits the queue — workers on different instances can't see each other's jobs. Redis breaks on RAM (queue depth), not CPU. If Redis backs up, it means workers aren't draining fast enough or something is broken. The fix is to increase the drain rate (more worker pods via HPA), not build a bigger dam (more Redis instances).
  - **Scaling chain**: `CPU spike → HPA adds worker pods → if nodes full, Cluster Autoscaler adds nodes → workers drain queue → CPU drops → HPA scales down → Cluster Autoscaler removes nodes`.
  - **Metrics Server**: Installed via Terraform Helm release in `kube-system` namespace. Required for HPA to read CPU/memory metrics from kubelet.
- **Ingress, Load Balancing & TLS**:
  - `infra/k8s/app/ingress.yaml` routes external HTTPS traffic to `vid2vec-api-service:8080`.
  - On AWS, `aws-load-balancer-controller` provisions an external Application Load Balancer (ALB) based on ingress annotations and terminates TLS via ACM.
  - On GCP, GKE Ingress controller terminates TLS and routes traffic to the backend service.
- **DNS & GoDaddy Handshake**:
  - Terraform manages Route53 (AWS) or Cloud DNS (GCP) hosted zones and outputs 4 NameServer (NS) addresses.
  - GoDaddy configuration is a **one-time** setup: update domain Custom Nameservers in GoDaddy to the 4 cloud NameServers.
  - All subsequent subdomain routing and ACM automated DNS certificate validation occur fully managed within AWS/GCP.
- **Database Engine Optimization & Production Sizing (pgvector)**:
  - **AWS**: `db.t4g.small` (2GB RAM), 7-day automated backup retention, final snapshot retained on delete.
  - **GCP**: `db-custom-1-3840` (3.75GB RAM), point-in-time recovery (PITR) enabled.
  - `maintenance_work_mem = 256MB`: Drastically accelerates HNSW / IVFFlat vector index creation.
  - `work_mem = 64MB`: Optimizes in-memory sorting for vector distance ranking queries (`ORDER BY embedding <-> query`).
  - `max_parallel_maintenance_workers = 2`: Enables parallel workers for vector index builds.

---

## 4. Container Image Build & Push

- `Dockerfile` is the local development image. It is intentionally left untouched.
- `Dockerfile.prod` is the production image: a multi-stage build that compiles every `./cmd/...` binary in a Go builder stage and ships only the binaries plus CA certificates in a `debian:bookworm-slim` runtime.
- The image reference lives in the Kustomize `images` transformer, not in the workload manifests. The pipeline overrides it at deploy time with `kustomize edit set image vid2vec=<registry>/vid2vec:<tag>`.
- Registry is provisioned by Terraform: ECR on AWS, Artifact Registry on GCP. Both expose their repository URL as a Terraform output consumed by the pipeline.

## 5. Deployment Pipeline Order (dependency-safe)

Deployment workflows (`.github/workflows/deploy-aws.yaml`, `.github/workflows/deploy-gcp.yaml`, and `.github/workflows/deploy.yaml`) are manual only (`workflow_dispatch`). The stages run in this fixed order, and each stage waits for the previous dependency before starting:

1. **Phase 1** - Terraform L1 (VPC, cluster, node autoscaling tags, OIDC, container registry, subnets/network, DNS hosted zone, ACM cert validation, Helm controllers).
2. **Phase 2** - Build & push `Dockerfile.prod` to the provisioned registry.
3. **Phase 3.1** - Configure kubeconfig (`aws eks update-kubeconfig` / `gcloud container clusters get-credentials`).
4. **Phase 3.2** - Dynamically inject tokens into Crossplane and Ingress manifests (`<AWS_ACCOUNT_ID>`, `<DOMAIN_NAME>`, `<ACM_CERTIFICATE_ARN>`, etc.) using caller identity, cluster identity, and Terraform outputs.
5. **Phase 3.3-3.5** - Apply Crossplane, wait for Providers `Healthy`, wait for the database `Ready` and for the `postgres-conn` connection secret.
6. **Phase 3.6** - Generate `vid2vec-db-credentials` Secret dynamically from `postgres-conn`.
7. **Phase 3.7-3.8** - Setup Kustomize and set image tag.
8. **Phase 3.9** - Apply the platform overlay (Redis with PVC, DB credentials, migration Job) and wait for Redis rollout and for the migration Job to complete.
9. **Phase 3.10** - Apply the app overlay (4 workers, API, HPA, Ingress) and wait for every rollout.

No stage is allowed to swallow a failure: there are no `|| true` fallbacks on rollout or wait commands.

## 6. Dependency Ordering (docker-compose `depends_on` equivalent)

Kubernetes has no native `depends_on`, so ordering is expressed with two mechanisms: init containers inside a pod, and explicit pipeline stages.

| docker-compose | Equivalent used here |
| :--- | :--- |
| `depends_on: [x]` (start order) | Pipeline stages: platform overlay is applied and waits complete before the app overlay is applied. |
| `condition: service_healthy` | `initContainers` on API and workers: `wait-for-redis` (redis-cli ping) and `wait-for-postgres` (pg_isready against `DATABASE_URL`). |
| `condition: service_completed_successfully` | Migration Job plus `kubectl wait --for=condition=complete job/vid2vec-db-migration` before the app overlay is applied. |
| restart / health gating | `startupProbe`, `livenessProbe`, `readinessProbe` on every container. |

Dependency graph:

```
registry  ->  (pipeline)  ->  image available
crossplane DB Ready + postgres-conn secret  ->  migration Job  ->  workers + API
redis Ready (PVC backed)  ->  workers + API
postgres Ready  ->  workers + API
ALB / Ingress Ready  ->  external HTTPS traffic routed to API
```

## 7. Drop-in Values

When deployed through `.github/workflows/deploy-aws.yaml` or `.github/workflows/deploy-gcp.yaml`, the pipeline automatically resolves and injects markers into Crossplane and Kubernetes manifests at runtime from repository secrets, variables, and Terraform outputs.

For manual out-of-band runs (`kubectl apply`), replace the placeholders below beforehand.

### Repository configuration (GitHub Actions)

| Name | Type | What it is |
| :--- | :--- | :--- |
| `AWS_REGION` | variable | AWS region, e.g. `us-east-1` |
| `GCP_REGION` | variable | GCP region, e.g. `us-central1` |
| `GCP_PROJECT_ID` | variable | GCP project ID (`gcloud config get-value project`) |
| `DOMAIN_NAME` | variable | Root domain name for DNS and Ingress (e.g. `example.com`) |
| `AWS_ACCESS_KEY_ID` | secret | AWS access key with registry + EKS permissions |
| `AWS_SECRET_ACCESS_KEY` | secret | Matching AWS secret key |
| `GCP_SA_KEY` | secret | GCP service account JSON key |
| `S3_MEDIA_BUCKET` | variable | Optional S3 media bucket name (defaults to `vid2vec-media-<AWS_ACCOUNT_ID>`) |

### Terraform

| File | Placeholder | What it must be |
| :--- | :--- | :--- |
| `infra/terraform/gcp/terraform.tfvars` | `project_id` | Real GCP project ID. Copy `terraform.tfvars.example` to `terraform.tfvars`. |
| `infra/terraform/gcp/terraform.tfvars` | `domain_name` | Domain name for Cloud DNS zone (defaults to `vid2vec.example.com`). |
| `infra/terraform/aws/terraform.tfvars` | `domain_name` | Domain name for Route53 and ACM (defaults to `vid2vec.example.com`). |

### Kubernetes and Crossplane

| File | Placeholder | What it must be |
| :--- | :--- | :--- |
| `infra/k8s/app/ingress.yaml` | `<DOMAIN_NAME>` | Target API domain (e.g. `api.example.com`). Auto-injected in CI from `DOMAIN_NAME`. |
| `infra/k8s/app/ingress.yaml` | `<ACM_CERTIFICATE_ARN>` | ACM Certificate ARN from Terraform outputs. Auto-injected in AWS CI. |
| `infra/k8s/platform/db-credentials.yaml` | `REPLACE_DB_USER`, `REPLACE_DB_PASSWORD`, `REPLACE_DB_HOST` | Full DSN. Compose from the Crossplane `postgres-conn` secret: `username`, `password`, `endpoint`. |
| `infra/k8s/platform/kustomization.yaml`, `infra/k8s/app/kustomization.yaml` | `placeholder.registry/vid2vec` | Registry image path. The pipeline overrides this; replace it for manual `kubectl apply -k`. |
| `infra/crossplane/aws/provider-aws.yaml`, `infra/crossplane/aws/iam.yaml` | `<AWS_ACCOUNT_ID>` | 12-digit AWS account ID (`aws sts get-caller-identity --query Account --output text`). |
| `infra/crossplane/aws/iam.yaml` | `<EKS_OIDC_ID>` | OIDC provider ID (`aws eks describe-cluster --name <cluster> --query cluster.identity.oidc.issuer --output text`). |
| `infra/crossplane/aws/iam.yaml` | `<S3_MEDIA_BUCKET>` | Media bucket name. |
| `infra/crossplane/aws/rds-postgres.yaml` | `<PRIVATE_SUBNET_ID_AZ_A>`, `<PRIVATE_SUBNET_ID_AZ_B>` | Two private subnet IDs in different AZs. |
| `infra/crossplane/gcp/provider-gcp.yaml`, `infra/crossplane/gcp/iam.yaml`, `infra/crossplane/gcp/cloudsql-postgres.yaml` | `<GCP_PROJECT_ID>` | Real GCP project ID. |
| `infra/crossplane/gcp/cloudsql-postgres.yaml` | `<GCP_NETWORK_NAME>` | VPC network name from the Terraform output. |
| `infra/crossplane/aws/iam.yaml`, `infra/crossplane/gcp/iam.yaml` | `john@example.com`, `james@example.com`, `dave@example.com` | Real human account emails. |

### GoDaddy Delegation Handshake

Once Terraform completes Phase 1:
1. Read the outputted nameservers:
   - AWS: `terraform output route53_name_servers`
   - GCP: `terraform output dns_name_servers`
2. Log into GoDaddy -> Domain Settings -> Manage DNS -> Nameservers -> **Change to Custom Nameservers**.
3. Paste the 4 cloud nameserver hostnames and save.
4. GoDaddy delegates all DNS resolution to the cloud provider. TLS certificate validation and API traffic routing proceed automatically.
