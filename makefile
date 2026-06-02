CONTAINER := discovery-engine-ingestion
EXEC      := docker exec $(CONTAINER)
EXECIT    := docker exec -it $(CONTAINER)

CMDS := api worker-analyzer worker-embedder worker-state worker-uploader

# ── Development ──────────────────────────────────────────────────────────────

build:              ## Compile all binaries into ./bin/
	cd /app && mkdir -p bin && \
	$(foreach cmd,$(CMDS),go build -buildvcs=false -o bin/$(cmd) ./cmd/$(cmd) &&) true

build-%:            ## Compile a single binary (e.g. make build-api)
	cd /app && mkdir -p bin && go build -buildvcs=false -o bin/$* ./cmd/$*

tidy:               ## go mod tidy
	cd /app && go mod tidy

fmt:                ## Format all Go files
	cd /app && go fmt ./...

vet:                ## Run go vet
	cd /app && go vet ./...

test:               ## Run all tests
	cd /app && go test ./...

lint:               ## vet + fmt check
	@$(MAKE) vet
	@$(MAKE) fmt

migrate:            ## Run database migrations
	cd /app && go run ./cmd/migrate

# ── PM2 ───────────────────────────────────────────────────────────────────────

start:              ## Start all workers + API via PM2 (foreground with logs)
	cd /app && pm2 start ecosystem.config.js && pm2 monit

start-d:            ## Start all workers + API via PM2 (daemon)
	cd /app && pm2 start ecosystem.config.js

start-io:            ## Start all workers + API via PM2 (foreground with logs, ignore stdin)
	cd /app && pm2 start ecosystem.config.js --no-daemon

restart:            ## Restart all PM2 processes
	pm2 restart all

kill:               ## Kill PM2 daemon and all processes
	pm2 kill

monit:              ## Open PM2 monitoring dashboard
	pm2 monit

pm2-logs:           ## Tail all PM2 logs
	pm2 logs

status:             ## Show PM2 process list
	pm2 status

# ── Docker (run from host) ────────────────────────────────────────────────────

d-build:            ## [docker] Compile all binaries
	$(EXEC) make build

d-tidy:             ## [docker] go mod tidy
	$(EXEC) make tidy

d-fmt:              ## [docker] Format all Go files
	$(EXEC) make fmt

d-vet:              ## [docker] Run go vet
	$(EXEC) make vet

d-test:             ## [docker] Run all tests
	$(EXEC) make test

d-lint:             ## [docker] vet + fmt check
	$(EXEC) make lint

d-migrate:          ## [docker] Run database migrations
	$(EXEC) make migrate

d-start:            ## [docker] Start all workers + API via PM2 (foreground with logs)
	$(EXECIT) make start

d-start-d:          ## [docker] Start all workers + API via PM2 (daemon)
	$(EXEC) make start-d

d-restart:          ## [docker] Restart all PM2 processes
	$(EXEC) make restart

d-kill:             ## [docker] Kill PM2 daemon and all processes
	$(EXEC) make kill

d-monit:            ## [docker] Open PM2 monitoring dashboard
	$(EXECIT) make monit

d-pm2-logs:         ## [docker] Tail all PM2 logs
	$(EXECIT) make pm2-logs

d-status:           ## [docker] Show PM2 process list
	$(EXEC) make status

shell:              ## Open a bash shell in the container
	$(EXECIT) bash

logs:               ## Tail container logs
	docker logs -f $(CONTAINER)

# ── Helpers ──────────────────────────────────────────────────────────────────

help:               ## Show this help
	@grep -E '^[a-zA-Z_%-]+:.*?## ' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36mmake %-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build tidy fmt vet test lint migrate \
        start start-d restart kill monit pm2-logs status \
        d-build d-tidy d-fmt d-vet d-test d-lint d-migrate \
        d-start d-start-d d-restart d-kill d-monit d-pm2-logs d-status \
        shell logs help

.DEFAULT_GOAL := help
