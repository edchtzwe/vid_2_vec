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
  - Best suited for core foundational infrastructure that changes infrequently: VPCs, subnets, routing tables, managed Kubernetes control planes (EKS/GKE), and base IAM OIDC providers.
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
| `infra/README.md` | Create | Architecture guide: Terraform vs Crossplane breakdown, K8s mental model, and artifact inventory table. |
| `infra/terraform/aws/main.tf` | Create | Root AWS module: VPC, EKS cluster (2 nodes min), OIDC provider, Crossplane Helm chart. |
| `infra/terraform/aws/variables.tf` | Create | AWS region, VPC CIDR, cluster name, node instance types, placeholder tags. |
| `infra/terraform/aws/outputs.tf` | Create | EKS cluster endpoint, OIDC ARN, Crossplane IRSA role ARN. |
| `infra/terraform/gcp/main.tf` | Create | Root GCP module: VPC, GKE cluster (2 nodes min), Workload Identity, Crossplane Helm chart. |
| `infra/terraform/gcp/variables.tf` | Create | GCP project ID placeholder, region, VPC subnets, machine types. |
| `infra/terraform/gcp/outputs.tf` | Create | GKE cluster endpoint, Workload Identity pool, Crossplane SA email. |
| `infra/crossplane/aws/provider-aws.yaml` | Create | Crossplane AWS Provider & ControllerConfig with IRSA authentication. |
| `infra/crossplane/aws/rds-postgres.yaml` | Create | Crossplane managed RDS PostgreSQL instance with custom `ParameterGroup` tuned for pgvector. |
| `infra/crossplane/aws/iam.yaml` | Create | Crossplane managed IAM roles/policies (Machine: admin, worker, readonly; Humans: John, James, Dave). |
| `infra/crossplane/gcp/provider-gcp.yaml` | Create | Crossplane GCP Provider & ControllerConfig with Workload Identity authentication. |
| `infra/crossplane/gcp/cloudsql-postgres.yaml` | Create | Crossplane managed Cloud SQL PostgreSQL instance with databaseFlags tuned for pgvector. |
| `infra/crossplane/gcp/iam.yaml` | Create | Crossplane managed GCP Service Accounts & IAM bindings (Machine: admin, worker, readonly; Humans: John, James, Dave). |
| `infra/k8s/redis.yaml` | Create | Redis standalone deployment & Service for worker queue bus. |
| `infra/k8s/workers.yaml` | Create | 4 worker deployments (`uploader`, `state`, `analyzer`, `embedder`) with queue bindings & anti-affinity across 2 nodes. |
| `infra/k8s/api.yaml` | Create | API service deployment, ClusterIP service, and DB migration job. |
| `infra/k8s/kustomization.yaml` | Create | Kustomize root wiring configs, secrets, and deployments together. |
| `.github/workflows/deploy.yaml` | Create | Manual `workflow_dispatch` pipeline executing Terraform, Crossplane (Cloud SQL/RDS + 3-tier IAM), and K8s workloads. |

---

## 3. High Availability & Worker Architecture

- **Failover**: Both AWS EKS and GCP GKE node groups are provisioned with a minimum count of 2 nodes spread across multiple Availability Zones / regions.
- **Worker Queues**: The 4 distinct background workers (`worker-uploader`, `worker-state`, `worker-analyzer`, `worker-embedder`) operate as separate Kubernetes Deployments (2 replicas each, 8 pods total) connecting to a shared Redis instance via Asynq. Hard `requiredDuringSchedulingIgnoredDuringExecution` anti-affinity ensures exactly 1 replica of each worker per node.
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
- **Scaling Architecture (HPA, No KEDA)**:
  - **Workers & API**: CPU-based HPA (target 70% utilization). Min 2 replicas, max 4. Scale-down stabilization window of 300s prevents flapping.
  - **Redis**: No HPA. Redis is a singleton queue bus. Scaling Redis horizontally splits the queue — workers on different instances can't see each other's jobs. Redis breaks on RAM (queue depth), not CPU. If Redis backs up, it means workers aren't draining fast enough or something is broken. The fix is to increase the drain rate (more worker pods via HPA), not build a bigger dam (more Redis instances).
  - **Why not KEDA**: KEDA watches queue depth and exposes it as a scaling metric. But our workers are CPU-bound (video analysis, embedding generation). A growing queue is a symptom of workers hitting CPU saturation — CPU-based HPA already handles that. KEDA adds infrastructure complexity without value when the bottleneck is compute, not queue length. If queue depth becomes the bottleneck in the future (e.g., workers are I/O-bound waiting on external APIs), KEDA can be added as a layer on top of HPA.
  - **Scaling chain**: `CPU spike → HPA adds worker pods → if nodes full, Cluster Autoscaler adds nodes → workers drain queue → CPU drops → HPA scales down → Cluster Autoscaler removes nodes`.
  - **Metrics Server**: Installed via Terraform Helm release in `kube-system` namespace. Required for HPA to read CPU/memory metrics from kubelet.
- **Database Engine Optimization (pgvector)**:
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

`.github/workflows/deploy.yaml` is manual only (`workflow_dispatch`). The stages run in this fixed order, and each stage waits for the previous dependency before starting:

1. **Phase 1** - Terraform L1 (VPC, cluster, OIDC, container registry, subnets/network).
2. **Phase 2** - Build & push `Dockerfile.prod` to the provisioned registry.
3. **Phase 3.1** - Configure kubeconfig (`aws eks update-kubeconfig` / `gcloud container clusters get-credentials`).
4. **Phase 3.2** - Dynamically inject tokens into Crossplane manifests using caller identity, cluster identity, and Terraform outputs.
5. **Phase 3.3-3.5** - Apply Crossplane, wait for Providers `Healthy`, wait for the database `Ready` and for the `postgres-conn` connection secret.
6. **Phase 3.6** - Generate `vid2vec-db-credentials` Secret dynamically from `postgres-conn`.
7. **Phase 3.7-3.8** - Setup Kustomize and set image tag.
8. **Phase 3.9** - Apply the platform overlay (Redis, DB credentials, migration Job) and wait for Redis rollout and for the migration Job to complete.
9. **Phase 3.10** - Apply the app overlay (4 workers, API) and wait for every rollout.

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
redis Ready  ->  workers + API
postgres Ready  ->  workers + API
```

## 7. Drop-in Values

When deployed through `.github/workflows/deploy.yaml`, the pipeline automatically resolves and injects the markers into Crossplane and Kubernetes manifests at runtime from repository secrets, variables, and Terraform outputs.

For manual out-of-band runs (`kubectl apply`), replace the placeholders below beforehand.

### Repository configuration (GitHub Actions)

| Name | Type | What it is |
| :--- | :--- | :--- |
| `AWS_REGION` | variable | AWS region, e.g. `us-east-1` |
| `GCP_REGION` | variable | GCP region, e.g. `us-central1` |
| `GCP_PROJECT_ID` | variable | GCP project ID (`gcloud config get-value project`) |
| `AWS_ACCESS_KEY_ID` | secret | AWS access key with registry + EKS permissions |
| `AWS_SECRET_ACCESS_KEY` | secret | Matching AWS secret key |
| `GCP_SA_KEY` | secret | GCP service account JSON key |
| `S3_MEDIA_BUCKET` | variable | Optional S3 media bucket name (defaults to `vid2vec-media-<AWS_ACCOUNT_ID>`) |

### Terraform

| File | Placeholder | What it must be |
| :--- | :--- | :--- |
| `infra/terraform/gcp/terraform.tfvars` | `project_id` | Real GCP project ID. Copy `terraform.tfvars.example` to `terraform.tfvars`. |
| `infra/terraform/aws/terraform.tfvars` | values | Optional overrides. Copy `terraform.tfvars.example` if you need non-defaults. |

### Kubernetes and Crossplane

| File | Placeholder | What it must be |
| :--- | :--- | :--- |
| `infra/k8s/platform/db-credentials.yaml` | `REPLACE_DB_USER`, `REPLACE_DB_PASSWORD`, `REPLACE_DB_HOST` | Full DSN. Compose from the Crossplane `postgres-conn` secret: `username`, `password`, `endpoint`. |
| `infra/k8s/platform/kustomization.yaml`, `infra/k8s/app/kustomization.yaml` | `placeholder.registry/vid2vec` | Registry image path. The pipeline overrides this; replace it for manual `kubectl apply -k`. |
| `infra/crossplane/aws/provider-aws.yaml`, `infra/crossplane/aws/iam.yaml` | `<AWS_ACCOUNT_ID>` | 12-digit AWS account ID (`aws sts get-caller-identity --query Account --output text`). |
| `infra/crossplane/aws/iam.yaml` | `<EKS_OIDC_ID>` | OIDC provider ID (`aws eks describe-cluster --name <cluster> --query cluster.identity.oidc.issuer --output text`). |
| `infra/crossplane/aws/iam.yaml` | `<S3_MEDIA_BUCKET>` | Media bucket name. |
| `infra/crossplane/aws/rds-postgres.yaml` | `<PRIVATE_SUBNET_ID_AZ_A>`, `<PRIVATE_SUBNET_ID_AZ_B>` | Two private subnet IDs in different AZs. |
| `infra/crossplane/gcp/provider-gcp.yaml`, `infra/crossplane/gcp/iam.yaml`, `infra/crossplane/gcp/cloudsql-postgres.yaml` | `<GCP_PROJECT_ID>` | Real GCP project ID. |
| `infra/crossplane/gcp/cloudsql-postgres.yaml` | `<GCP_NETWORK_NAME>` | VPC network name from the Terraform output. |
| `infra/crossplane/aws/iam.yaml`, `infra/crossplane/gcp/iam.yaml` | `john@example.com`, `james@example.com`, `dave@example.com` | Real human account emails. |
