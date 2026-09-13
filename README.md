# vid_2_vec

Video ingestion pipeline that uploads video to Google Files API, analyzes it with Gemini, and stores vector embeddings for semantic search.

## Architecture

```
POST /ingest/:id
       │
       ▼
  ┌──────────┐    ┌──────────────┐    ┌───────────┐    ┌──────────┐
  │ Uploader  │───▶│ State Checker │───▶│ Analyzer  │───▶│ Embedder │
  └──────────┘    └──────────────┘    └───────────┘    └──────────┘
  Upload video    Poll file state     Gemini watches    Generate 768d
  to Google       until ACTIVE        video, returns    vectors, store
  Files API                           scene metadata    in PostgreSQL
```

Four async workers connected via Redis (Asynq):

| Worker | Queue | What it does |
| :--- | :--- | :--- |
| **Uploader** | `upload` | Sends `.mp4` to Google Files API, records the provider file ID |
| **State Checker** | `state` | Polls Files API until processing completes (ACTIVE / FAILED) |
| **Analyzer** | `analyze` | Sends video to Gemini for structured scene analysis, writes manifest |
| **Embedder** | `embed` | Converts scene descriptions into 768-dim vectors, stores in PostgreSQL |

## Tech Stack

- **Go 1.25** + Echo (HTTP) + Asynq (task queue)
- **PostgreSQL 16** with pgvector for vector storage
- **Redis 7** for Asynq job queue
- **Google Gemini** for video analysis and embeddings
- **Google Files API** for video upload

## API

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/ingest/:id` | Start ingestion for a source video |
| `GET` | `/job-info/:id` | Check job status |
| `POST` | `/manual-embed/:id` | Re-embed a completed job |
| `GET` | `/health-check` | Liveness probe |

## Job Lifecycle

```
PENDING → UPLOADED_TO_PROVIDER → PROVIDER_PROCESSING → READY_FOR_ANALYSIS
→ ANALYZING → READY_FOR_EMBEDDING → EMBEDDING → COMPLETED
```

Any step can transition to `FAILED`.

## Running Locally

```bash
cp .env.example .env
# Edit .env with your GOOGLE_AI_KEY

docker compose up -d postgres redis
go run cmd/migrate/main.go
go run cmd/api/main.go
```

Workers (each in a separate terminal):

```bash
go run cmd/worker-uploader/main.go
go run cmd/worker-state/main.go
go run cmd/worker-analyzer/main.go
go run cmd/worker-embedder/main.go
```

## Tests

```bash
go test ./... -v
```

Unit tests mock all external dependencies (database, Gemini, Asynq). No real API calls needed.

CI runs automatically on every PR via GitHub Actions.

## Project Structure

```
cmd/
  api/                 HTTP server (Echo)
  worker-uploader/     Upload worker
  worker-state/        State checker worker
  worker-analyzer/     Analysis worker
  worker-embedder/     Embedding worker
  migrate/             Database migrations
internal/
  bootstrap/           Dependency wiring
  config/              Environment config
  gemini/              Google Gemini + Files API client
  handlers/            HTTP handlers
  repository/          Database layer (PostgreSQL)
  routes/              Route registration
  service/             Business logic + logging
  tasks/               Asynq task definitions
  workers/ai/          Worker implementations
infra/                 Terraform, Crossplane, K8s manifests
migrations/            SQL migration files
```
