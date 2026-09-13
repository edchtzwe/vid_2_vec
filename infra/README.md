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
