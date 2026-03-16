# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

github-hub is a Go application with three binaries:
- `ghh-server`: HTTP server that mirrors/caches GitHub repositories and packages
- `ghh`: CLI client that requests downloads and manages the server-side cache
- `quality-server`: GitHub webhook handler for quality checks (push/PR events)

Designed for environments without direct internet access.

## Build & Run Commands

```bash
# Build binaries
go build -o bin/ghh ./cmd/ghh
go build -o bin/ghh-server ./cmd/ghh-server
go build -o bin/quality-server ./cmd/quality-server

# Run client
go run ./cmd/ghh --help

# Run ghh-server (GITHUB_TOKEN optional but recommended for rate limits)
GITHUB_TOKEN=... bin/ghh-server --addr :8080 --root data

# Run quality-server (with optional MySQL)
bin/quality-server --addr :5001
bin/quality-server --addr :5001 --db "root:password@tcp(localhost:3306)/github_hub"

# Run frontend (requires build first)
cd frontend && npm install && npm run build

# Run tests with race detection and coverage
go test ./... -race -cover

# Run single test
go test -v -run TestName ./internal/server/

# Code checks
go vet ./...
go fmt ./...

# Using Make (recommended)
make build              # Build both server and client
make build-server       # Build server only
make build-client       # Build client only
make build-static       # Build static binaries
make build-cross        # Cross-platform builds (linux/darwin/windows × amd64/arm64)
make run                # Build and run server
make test               # Run tests with coverage
make clean              # Remove bin/ directory
```

## Architecture

```
cmd/
├── ghh/main.go           # CLI client entry point
├── ghh-server/main.go    # HTTP server entry point
└── quality-server/main.go # Quality engine webhook server

internal/
├── client/client.go      # HTTP client for ghh CLI → server communication
├── server/server.go      # HTTP handlers + janitor (cleanup goroutine)
├── storage/storage.go    # Workspace storage: downloads from GitHub, caches zips
├── config/config.go      # Client YAML/JSON config loader
├── version/version.go    # Version string (set via ldflags)
└── quality/              # Quality engine for GitHub webhook processing
    ├── api/server.go     # HTTP handlers for webhooks and events
    ├── handlers/         # Push and PR event handlers
    ├── models/           # Event and quality check models
    └── storage/          # MySQL storage for events
```

**Key flows:**
- **Git mode download**: Client → `GET /api/v1/download?repo=...` → Server checks `git-cache/` → If missing, `git clone --bare` → `git archive` streams back
- **Sparse download**: `GET /api/v1/download/sparse?repo=...&paths=src,docs` → Uses shared `git-cache/` bare repo for fast partial exports
- **Storage layout**:
  - User downloads: `<root>/users/<user>/repos/<owner>/<repo>/<branch>.zip` with `.meta` (SHA) and `.commit.txt` files
  - Git cache: `<root>/git-cache/<owner>/<repo>.git` (shared bare repos, supports `git fetch` updates)
  - Packages: `<root>/users/<user>/packages/<url-hash>/<filename>` keyed by SHA256 of URL
- **Janitor**: Background goroutine runs every minute, deletes items idle >24h
- **Quality server**: Receives GitHub webhooks (`/webhook`), filters events (main branch only), stores in file or MySQL, creates quality checks

**API endpoints** (in `internal/server/server.go`):
- `GET /api/v1/download` - download repo zip (git mode by default)
- `GET /api/v1/download/sparse` - download specific directories via git archive
- `GET /api/v1/download/commit` - get cached commit SHA
- `GET /api/v1/download/package` - download arbitrary URL with server-side caching
- `POST /api/v1/branch/switch` - ensure branch exists in cache
- `GET /api/v1/dir/list` - list directory contents
- `DELETE /api/v1/dir` - delete path from cache

**Quality engine endpoints** (in `internal/quality/api/server.go`):
- `POST /webhook` - receive GitHub webhooks (push, pull_request)
- `GET /api/events` - list stored events with filtering
- `GET /api/events/:id` - get event details
- `GET /api/events/:id/quality-checks` - list quality checks for event
- `PUT /api/quality-checks/:id` - update quality check status
- `POST /api/custom-test` - submit custom test events

## Code Conventions

- Use `gofmt`/`goimports`; no format differences before commit
- Error variable: `err`; wrap with `%w`; check with `errors.Is/As`
- Context as first parameter: `ctx context.Context`
- Prefer table-driven tests with standard `testing` package
- Commits follow Conventional Commits: `feat:`, `fix:`, `chore:`

## Configuration

**Client** (`--config` or `GHH_CONFIG`): YAML with `base_url`, `token`, `user`
**Server** (`--config`): YAML with `addr`, `root`, `default_user`, `token`, `download_timeout`
**Environment variables**: `GITHUB_TOKEN` (server), `GHH_BASE_URL`/`GHH_TOKEN`/`GHH_USER` (client)

## Docker Deployment

The project runs 4 containers in production:

1. **ghh-server** - GitHub repository caching server (port 8080)
   - Dockerfile: `Dockerfile` (multi-stage build with golang:1.24-alpine)
   - Base image: `alpine:3.19`
   - Volume: `/data` for cache storage

2. **quality-server** - GitHub webhook quality engine (port 5001)
   - Dockerfile: `Dockerfile_quality_server`
   - Base image: `alpine:3.19`
   - Requires database connection (MySQL)
   - Webhook endpoint: `/webhook`

3. **quality-frontend** - Web UI for quality engine (port 80)
   - Dockerfile: `Dockerfile_frontend`
   - Base image: `nginx:latest`
   - Serves static files from `frontend/dist`

4. **quality-mysql** - MySQL database for quality-server
   - Database: `github_hub`
   - Tables: `github_events`, `pr_quality_checks`
   - Init script: `scripts/init-mysql.sql`

**Important**: Pull base images from `docker-images` registry instead of Docker Hub:
```bash
# Example: Pull from local registry
docker pull docker-images/library/nginx:latest
docker pull docker-images/library/alpine:3.19
docker pull docker-images/library/golang:1.24-alpine
docker pull docker-images/library/mysql:latest
```

## Deployment Scripts

Two scripts are provided for container deployment:

### `prepare.sh` - Preparation Script
Pulls base images, builds binaries locally, and creates Docker images:
```bash
./prepare.sh
```
Steps:
1. Pulls base images from `docker-images` registry (nginx, alpine, golang, mysql)
2. Compiles Go binaries (ghh-server, ghh, quality-server) to `bin/`
3. Builds frontend (npm install + build) to `frontend/dist`
4. Builds Docker images (ghh-server, quality-server, quality-frontend)
5. Creates data directories and Docker network `github-hub-network`

### `deploy.sh` - Deployment Script
Starts all 4 containers using `docker run` (no docker-compose required):
```bash
./deploy.sh
```
Containers:
- `quality-mysql` (port 3306) - MySQL with init script
- `quality-server` (port 5001) - Connects to quality-mysql
- `quality-frontend` (port 8081) - Nginx, proxies to quality-server
- `ghh-server` (port 8080) - Standalone caching server

**Note**: Containers are started in dependency order, with health checks.

## Testing

Test files: `*_test.go` alongside source. Key test files:
- `internal/server/server_test.go` - API handler tests with fake store
- `internal/server/server_download_test.go` - download-specific tests
- `internal/storage/storage_test.go` - storage layer tests
- `cmd/ghh/main_test.go` - CLI integration tests
- `internal/quality/models/event_test.go` - Quality event model tests (18 tests, 94.1% coverage)
- `internal/quality/handlers/pr_handler_test.go` - PR event handler tests (9 tests)
- `internal/quality/handlers/push_handler_test.go` - Push event handler tests (13 tests)
- `internal/quality/storage/storage_test.go` - Storage layer tests with MockStorage
- `internal/quality/storage/mock.go` - Mock storage implementation for testing

**Quality module test coverage**:
- Models: Event creation, quality check generation, event filtering logic
- Handlers: Push/PR event processing for both simplified and webhook formats
- Storage: Mock storage with CRUD operations, error handling, cleanup

Run single test: `go test -v -run TestName ./internal/server/`

Run quality tests: `go test ./internal/quality/... -race -cover`
