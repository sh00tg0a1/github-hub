# GitHub-Hub

A comprehensive GitHub toolkit for offline environments consisting of two main components:

1. **ghh-server** - GitHub repository caching server for offline downloads
2. **quality-server** - GitHub Webhook quality check service for automated CI/CD

---

## Table of Contents

- [ghh-server: GitHub Cache Server](#ghh-server-github-cache-server)
  - [Features](#features)
  - [Quick Start](#quick-start-1)
  - [Deployment](#deployment)
  - [CLI Reference](#cli-reference)
  - [HTTP API](#http-api)
- [quality-server: Quality Check Service](#quality-server-quality-check-service)
  - [Features](#features-1)
  - [System Architecture](#system-architecture)
  - [Quick Start](#quick-start-2)
  - [API Reference](#api-reference)
- [Development](#development)
- [Testing](#testing)
- [Load Testing](#load-testing)

---

# ghh-server: GitHub Cache Server

A lightweight HTTP server and CLI client that mirrors GitHub repositories into an offline-friendly cache.

## Features

- **Offline caching**: Download and cache GitHub repositories as ZIP files
- **Sparse checkout**: Download only specific directories from large repositories
- **Git mode**: Uses bare Git repositories with `git archive` for faster downloads and cache reuse
- **Web UI**: Browse and manage cached repositories
- **User isolation**: Multi-user support with separated cache namespaces
- **Auto cleanup**: Automatically removes entries idle >24 hours

## Quick Start

```bash
# 1. Start server
go run ./cmd/ghh-server

# 2. Download repository (new terminal)
go run ./cmd/ghh download --repo owner/repo --dest out.zip
```

### Deployment Options

**Native Build**
```bash
go build -o bin/ghh-server ./cmd/ghh-server
go build -o bin/ghh ./cmd/ghh
GITHUB_TOKEN=<optional> bin/ghh-server --addr :8080 --root data
```

**Docker**
```bash
docker build -t ghh-server .
docker run -p 8080:8080 -v $(pwd)/data:/data -e GITHUB_TOKEN=your_token ghh-server
```

**Make (Recommended)**
```bash
make build   # Build both server and client
make run     # Build and run server on :8080
```

## CLI Reference

### Server (ghh-server)

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--addr` | - | `:8080` | Listen address |
| `--root` | - | `data` | Cache root directory |
| `--config` | - | - | Server config file path |
| - | `GITHUB_TOKEN` | - | GitHub API token |

### Client (ghh)

#### Global Options

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--server` | `GHH_BASE_URL` | `http://localhost:8080` | Server address |
| `--token` | `GHH_TOKEN` | - | Auth token |
| `--user` | `GHH_USER` | `default` | User name |

#### Commands

**download** - Download repository (ZIP or extracted)
```bash
ghh download --repo <owner/repo> --dest <path> [options]
```

| Flag | Description |
|------|-------------|
| `--repo` | Repository identifier (required) |
| `--branch` | Branch name (default: main) |
| `--dest` | Destination path |
| `--extract` | Extract to directory |
| `--legacy` | Use legacy GitHub API instead of git archive |

**download-sparse** - Download specific directories only
```bash
ghh download-sparse --repo <owner/repo> [--path <dir>] [--dest <path>]
```

| Flag | Description |
|------|-------------|
| `--repo` | Repository identifier (required) |
| `--path` | Directory to include (can specify multiple times) |
| `--branch` | Branch name (default: main) |
| `--extract` | Extract to directory |

**switch** - Pre-cache a branch
```bash
ghh switch --repo <owner/repo> --branch <branch>
```

**ls** - List server cache
```bash
ghh ls [--path <path>]
```

**rm** - Delete cache
```bash
ghh rm --path <path> [-r]
```

## HTTP API

### Download Repository

```bash
# GET /api/v1/download
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo&branch=main"
```

| Param | Required | Description |
|-------|----------|-------------|
| `repo` | ✅ | Repository identifier (`owner/repo`) |
| `branch` | ❌ | Branch name |
| `user` | ❌ | User name |

### Sparse Download

```bash
# GET /api/v1/download/sparse
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src,docs"
```

| Param | Required | Description |
|-------|----------|-------------|
| `repo` | ✅ | Repository identifier |
| `paths` | ✅ | Comma-separated directory list |
| `branch` | ❌ | Branch name (default: main) |

### Branch Switch

```bash
# POST /api/v1/branch/switch
curl -X POST "http://localhost:8080/api/v1/branch/switch" \
  -H "Content-Type: application/json" \
  -d '{"repo": "owner/repo", "branch": "dev"}'
```

### List Directory

```bash
# GET /api/v1/dir/list
curl "http://localhost:8080/api/v1/dir/list?path=repos/owner/repo"
```

### Delete

```bash
# DELETE /api/v1/dir
curl -X DELETE "http://localhost:8080/api/v1/dir?path=repos/owner/repo&recursive=true"
```

---

# quality-server: Quality Check Service

GitHub Webhook quality check service for monitoring and processing GitHub Pull Request and Push events with automated quality checks.

## Features

- **GitHub Webhook Integration**: Receives and processes GitHub PR and Push events
- **Smart Event Filtering**: Only processes PRs to main branch and main branch pushes
- **Multi-stage Quality Checks**: Supports Basic CI, Deployment, and Specialized Tests
- **MySQL Persistence**: Stores events and quality check data
- **RESTful API**: Complete API for querying and managing data
- **Docker Support**: Containerized deployment
- **Mock Testing**: Predefined and custom test support

## System Architecture

### Container Architecture

The project includes 4 Docker containers:

| Container | Port | Description |
|-----------|------|-------------|
| **ghh-server** | 8080 | GitHub repository cache server |
| **quality-server** | 5001 | GitHub webhook quality engine |
| **quality-frontend** | 8081 | Web UI |
| **quality-mysql** | 3307 | MySQL database |

### Quality Check Stages

1. **Basic CI**
   - Compilation check
   - Code linting
   - Security scan
   - Unit tests

2. **Deployment**
   - Deployment check

3. **Specialized Tests**
   - API tests
   - Module E2E tests
   - Agent E2E tests
   - AI E2E tests

## Quick Start

```bash
# 1. Prepare - Load images, build binaries, build Docker images
./prepare.sh

# 2. Deploy - Start all 4 containers
./deploy.sh
```

This starts:
- **ghh-server** (port 8080) - GitHub cache server
- **quality-server** (port 5001) - Quality engine API
- **quality-frontend** (port 8081) - Web UI
- **quality-mysql** (port 3307) - Database

**MySQL Connection**:
- Database: `github_hub`
- Username: `root`
- Password: `root123456`

## API Reference

### Webhook Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/webhook` | Receive GitHub Webhook events |

### Event Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/events` | Get event list |
| `GET` | `/api/events/:id` | Get event details |
| `PUT` | `/api/events/:id/status` | Update event status |
| `DELETE` | `/api/events` | Delete all events |

#### Update Event Status

Update the status of an event.

```bash
# PUT /api/events/:id/status
curl -X PUT "http://localhost:5001/api/events/1/status" \
  -H "Content-Type: application/json" \
  -d '{
    "event_status": "completed",
    "processed_at": "2026-02-10T10:30:00Z"
  }'
```

**Request Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `event_status` | string | ❌ | Status: `pending`, `processing`, `completed`, `failed` |
| `processed_at` | string | ❌ | Processing completion time (ISO 8601 format) |

**Response:**
```json
{
  "success": true,
  "message": "事件状态更新成功",
  "data": {
    "id": 1,
    "event_id": "evt-123",
    "event_type": "push",
    "event_status": "completed",
    "processed_at": "2026-02-10T10:30:00Z",
    "repository": "owner/repo",
    "branch": "main",
    "commit_sha": "abc123",
    "created_at": "2026-02-10T10:00:00Z",
    "updated_at": "2026-02-10T10:30:00Z"
  }
}
```

### Quality Checks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/events/:eventID/quality-checks` | Get quality check list |
| `PUT` | `/api/quality-checks/:id` | Update quality check status |
| `PUT` | `/api/events/:eventID/quality-checks/batch` | Batch update quality checks |

#### Update Quality Check Status

Update a single quality check by ID.

```bash
# PUT /api/quality-checks/:id
curl -X PUT "http://localhost:5001/api/quality-checks/1" \
  -H "Content-Type: application/json" \
  -d '{
    "check_status": "passed",
    "error_message": null,
    "output": "All tests passed successfully",
    "duration_seconds": 15.5
  }'
```

**Request Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `check_status` | string | ❌ | Status: `pending`, `running`, `passed`, `failed`, `skipped`, `cancelled` |
| `error_message` | string | ❌ | Error message (if failed) |
| `output` | string | ❌ | Output log |
| `duration_seconds` | number | ❌ | Duration in seconds |

**Response:**
```json
{
  "success": true,
  "data": {
    "id": 1,
    "github_event_id": "evt-123",
    "check_type": "compilation",
    "check_status": "passed",
    "stage": "basic_ci",
    "stage_order": 1,
    "check_order": 1,
    "started_at": "2026-02-10T10:00:00Z",
    "completed_at": "2026-02-10T10:00:15Z",
    "duration_seconds": 15.5,
    "error_message": null,
    "output": "All tests passed successfully",
    "retry_count": 0,
    "created_at": "2026-02-10T10:00:00Z",
    "updated_at": "2026-02-10T10:00:15Z"
  }
}
```

#### Batch Update Quality Checks

Update multiple quality checks for an event. When all checks are completed, the event status is automatically updated to `completed`.

```bash
# PUT /api/events/:eventID/quality-checks/batch
curl -X PUT "http://localhost:5001/api/events/1/quality-checks/batch" \
  -H "Content-Type: application/json" \
  -d '{
    "quality_checks": [
      {
        "id": 1,
        "check_status": "passed",
        "started_at": "2026-02-10T10:00:00Z",
        "completed_at": "2026-02-10T10:00:05Z",
        "duration_seconds": 5.0,
        "error_message": null,
        "output": "Compilation successful"
      },
      {
        "id": 2,
        "check_status": "passed",
        "started_at": "2026-02-10T10:00:05Z",
        "completed_at": "2026-02-10T10:00:08Z",
        "duration_seconds": 3.0,
        "error_message": null,
        "output": "Code lint passed"
      },
      {
        "id": 3,
        "check_status": "failed",
        "started_at": "2026-02-10T10:00:08Z",
        "completed_at": "2026-02-10T10:00:15Z",
        "duration_seconds": 7.0,
        "error_message": "Test failed: assertion error",
        "output": "Running tests...\nTest 1: PASS\nTest 2: FAIL"
      }
    ]
  }'
```

**Request Parameters:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `quality_checks` | array | ✅ | Array of quality check updates |
| `quality_checks[].id` | number | ✅ | Quality check ID |
| `quality_checks[].check_status` | string | ❌ | Status: `pending`, `running`, `passed`, `failed`, `skipped`, `cancelled` |
| `quality_checks[].started_at` | string | ❌ | Start time (ISO 8601 format) |
| `quality_checks[].completed_at` | string | ❌ | Completion time (ISO 8601 format) |
| `quality_checks[].duration_seconds` | number | ❌ | Duration in seconds |
| `quality_checks[].error_message` | string | ❌ | Error message (if failed) |
| `quality_checks[].output` | string | ❌ | Output log |

**Response:**
```json
{
  "success": true,
  "message": "成功更新 3 个质量检查项",
  "data": [
    {
      "id": 1,
      "github_event_id": "evt-123",
      "check_type": "compilation",
      "check_status": "passed",
      "stage": "basic_ci",
      "stage_order": 1,
      "check_order": 1,
      "started_at": "2026-02-10T10:00:00Z",
      "completed_at": "2026-02-10T10:00:05Z",
      "duration_seconds": 5.0,
      "error_message": null,
      "output": "Compilation successful",
      "retry_count": 0,
      "created_at": "2026-02-10T10:00:00Z",
      "updated_at": "2026-02-10T10:00:15Z"
    },
    {
      "id": 2,
      "github_event_id": "evt-123",
      "check_type": "code_lint",
      "check_status": "passed",
      "stage": "basic_ci",
      "stage_order": 1,
      "check_order": 2,
      "started_at": "2026-02-10T10:00:05Z",
      "completed_at": "2026-02-10T10:00:08Z",
      "duration_seconds": 3.0,
      "error_message": null,
      "output": "Code lint passed",
      "retry_count": 0,
      "created_at": "2026-02-10T10:00:00Z",
      "updated_at": "2026-02-10T10:00:15Z"
    },
    {
      "id": 3,
      "github_event_id": "evt-123",
      "check_type": "security_scan",
      "check_status": "failed",
      "stage": "basic_ci",
      "stage_order": 1,
      "check_order": 3,
      "started_at": "2026-02-10T10:00:08Z",
      "completed_at": "2026-02-10T10:00:15Z",
      "duration_seconds": 7.0,
      "error_message": "Test failed: assertion error",
      "output": "Running tests...\nTest 1: PASS\nTest 2: FAIL",
      "retry_count": 0,
      "created_at": "2026-02-10T10:00:00Z",
      "updated_at": "2026-02-10T10:00:15Z"
    }
  ]
}
```

### Mock Test Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/mock/events` | Get mock event templates |
| `POST` | `/api/mock/simulate/:event-type` | Simulate predefined event |
| `POST` | `/api/custom-test` | Execute custom test |

### Other Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/repositories` | Get repository list |
| `GET` | `/api/status` | Get system status |
| `POST` | `/api/login` | User login |
| `POST` | `/api/logout` | User logout |
| `GET` | `/api/check-login` | Check login status |

## Event Filtering Rules

### Push Events
Only processes main branch pushes. Other branches are ignored.

### Pull Request Events
Only processes PRs from non-main branches to main branch. Other PRs are ignored.

## Database Schema

### github_events Table

| Field | Type | Description |
|-------|------|-------------|
| id | INT | Primary key |
| event_id | VARCHAR(36) | Event unique identifier |
| event_type | VARCHAR(50) | Event type |
| event_status | VARCHAR(50) | Event status |
| repository | VARCHAR(255) | Repository name |
| branch | VARCHAR(255) | Branch name |
| target_branch | VARCHAR(255) | Target branch |
| commit_sha | VARCHAR(255) | Commit SHA |
| pr_number | INT | PR number |
| action | VARCHAR(50) | Action type |
| pusher | VARCHAR(255) | Pusher |
| author | VARCHAR(255) | Author |
| payload | JSON | Event payload |
| created_at | TIMESTAMP | Created at |
| updated_at | TIMESTAMP | Updated at |
| processed_at | TIMESTAMP | Processed at |

### pr_quality_checks Table

| Field | Type | Description |
|-------|------|-------------|
| id | INT | Primary key |
| github_event_id | VARCHAR(36) | Associated event ID |
| check_type | VARCHAR(50) | Check type |
| check_status | VARCHAR(50) | Check status |
| stage | VARCHAR(50) | Check stage |
| stage_order | INT | Stage order |
| check_order | INT | Check order |
| started_at | TIMESTAMP | Started at |
| completed_at | TIMESTAMP | Completed at |
| duration_seconds | DOUBLE | Duration in seconds |
| error_message | TEXT | Error message |
| output | TEXT | Output |
| retry_count | INT | Retry count |
| created_at | TIMESTAMP | Created at |
| updated_at | TIMESTAMP | Updated at |

---

# Shell Scripts Reference

This project includes several shell scripts for building, testing, deploying, and managing the application.

## Development Scripts

### prepare.sh
Builds all components and prepares Docker images.

```bash
./prepare.sh
```

**What it does:**
1. Loads base Docker images (alpine, golang, nginx, mysql)
2. Compiles Go binaries (ghh-server, ghh, quality-server)
3. Builds React frontend
4. Builds Docker images for all services

**Output:**
- `bin/ghh-server` - ghh-server binary
- `bin/ghh` - ghh CLI binary
- `bin/quality-server` - quality-server binary
- Docker images: ghh-server:latest, quality-server:latest, quality-frontend:latest

---

### test.sh
Runs unit tests for all Go modules.

```bash
# Run all tests
./test.sh

# Verbose output (shows each test command)
./test.sh -v

# Show coverage report
./test.sh -c

# Run with race detection
./test.sh -r

# Test specific module
./test.sh ./internal/quality

# Combined options
./test.sh -v -r -c
```

**What it does:**
- Runs `go test` on all Go packages
- Supports verbose mode showing test commands
- Optionally calculates code coverage
- Optionally enables race detection
- Tests 8 modules: cmd/ghh, cmd/ghh-server, internal/client, internal/server, internal/storage, internal/quality/api, internal/quality/handlers, internal/quality/storage

**Test Results:**
```
✓ cmd/ghh                  PASS
✓ cmd/ghh-server           PASS
✓ internal/client          PASS
✓ internal/server          PASS
✓ internal/storage         PASS
✓ internal/quality/api     PASS
✓ internal/quality/handlers PASS
✓ internal/quality/storage PASS

All tests passed! (8/8 modules)
```

---

### loadtest.sh
Load testing tool for quality-server webhook handling.

```bash
# Basic test (100 requests, 10 concurrent)
./loadtest.sh basic

# Moderate load (500 requests, 20 concurrent)
./loadtest.sh moderate

# Heavy load (1000 requests, 50 concurrent)
./loadtest.sh heavy

# Stress test (5000 requests, 100 concurrent)
./loadtest.sh stress

# Extreme test (10000 requests, 200 concurrent)
./loadtest.sh extreme

# PR event test
./loadtest.sh pr

# Rate-limited test (500 requests, 100 QPS)
./loadtest.sh rate

# Custom parameters
./loadtest.sh custom -n 1000 -c 50 -type push

# Specify server
QUALITY_SERVER_URL=http://localhost:5001 ./loadtest.sh stress
```

**What it does:**
- Compiles the load testing tool (first run only)
- Sends concurrent webhook requests to quality-server
- Measures throughput, latency, and success rate
- Supports both push and PR event types

---

### test_webhook.sh
Simulates GitHub webhook events for testing quality-server.

```bash
# Run all tests (default server: http://10.4.174.125:5001)
./test_webhook.sh

# Specify custom server
QUALITY_SERVER_URL=http://localhost:5001 ./test_webhook.sh
```

**What it does:**
- Sends Push event (main branch)
- Sends Pull Request event (feature → main)
- Verifies event processing
- Displays response status

---

## Deployment Scripts

### save-images.sh
Exports Docker images to tar files for offline deployment.

```bash
./save-images.sh
```

**What it does:**
1. Saves Docker images to `docker-images-export/` directory
2. Creates tar files: ghh-server_latest.tar, quality-server_latest.tar, quality-frontend_latest.tar, mysql_latest.tar, nginx_latest.tar, alpine_3.19.tar

**Output:**
```
docker-images-export/
├── alpine_3.19.tar (7.4M)
├── ghh-server_latest.tar (20M)
├── mysql_latest.tar (901M)
├── nginx_latest.tar (157M)
├── quality-frontend_latest.tar (157M)
└── quality-server_latest.tar (21M)
```

**Use case:**
- Preparing images for transfer to air-gapped servers
- Creating image backups
- Offline deployment scenarios

---

### start.sh
Loads Docker images and starts all containers on target server.

```bash
# Upgrade mode (default): update containers, preserve data
./start.sh

# Recover mode: complete reinstall, clear database
./start.sh -r

# Show help
./start.sh -h
```

**What it does:**

**Pre-checks:**
- Checks port availability (3306, 5001, 8080, 8081)
- Shows container status (running/exited/nonexistent)
- Detects port conflicts

**Upgrade mode (-u or default):**
1. Loads Docker images from `docker-images-export/`
2. Creates Docker network (github-hub-network)
3. Creates data directories
4. Starts containers (skips if already running with same image)
5. Displays service status

**Recover mode (-r):**
- Stops and removes all containers
- Backs up database to `data/mysql.backup.YYYYMMDD_HHMMSS`
- Resets database directory
- Recreates Docker network
- Performs fresh installation

**Services started:**
| Container | Port | Description |
|-----------|------|-------------|
| quality-mysql | 3306 | MySQL database |
| quality-server | 5001 | Quality API server |
| ghh-server | 8080 | GitHub cache server |
| quality-frontend | 8081 | Web UI |

**Access URLs:**
```
Frontend:       http://<server-ip>:8081
Quality API:    http://<server-ip>:5001
GHH Server:     http://<server-ip>:8080
MySQL:          localhost:3306
```

**Database credentials:**
- Host: quality-mysql
- Port: 3306
- Database: github_hub
- Username: root
- Password: root123456

---

### stop.sh
Stops and removes all containers.

```bash
./stop.sh
```

**What it does:**
- Stops containers: quality-frontend, quality-server, ghh-server, quality-mysql
- Removes containers
- Preserves data volumes

---

### deploy.sh
Local deployment script (development use).

```bash
./deploy.sh
```

**What it does:**
- Checks container status
- Starts containers if not running
- Shows container status after deployment
- For local development only

**Note:** For production deployment, use `save-images.sh` + `start.sh` on target server.

---

### install_quality.sh

Quality project standalone deployment script for deploying Quality service independently without affecting the GHH project.

```bash
# Upgrade mode (default): update containers, preserve data
./install_quality.sh

# Upgrade mode
./install_quality.sh -u

# Recover mode: full reinstall, clear database
./install_quality.sh -r

# Show help
./install_quality.sh -h
```

**What it does:**

**Upgrade mode (-u or default):**
1. Compiles quality-server directly in Dockerfile (multi-stage build)
2. Builds quality-frontend image (builds frontend if frontend/dist doesn't exist)
3. Creates Docker network (quality-network)
4. Creates data directories
5. Starts containers (skips if already running)
6. Shows service status

**Recover mode (-r):**
- Stops and removes all Quality-related containers
- Backs up database to `data/quality-mysql.backup.YYYYMMDD_HHMMSS`
- Resets database directory
- Recreates Docker network
- Performs fresh installation

**Services started:**
| Container | Port | Description |
|-----------|------|-------------|
| quality-mysql | 3306 | MySQL database |
| quality-server | 5001 | Quality API server |
| quality-frontend | 8081 | Web interface |

**Difference from GHH project:**
- Separate Docker network: `quality-network` (doesn't affect GHH's `github-hub-network`)
- Separate data directories: `data/quality-mysql`, `data/quality-server`
- Can coexist with GHH project on the same server
- Uses multi-stage build to compile Go code directly, no pre-compiled binaries needed

**Access URLs:**
```
Frontend:       http://<server-ip>:8081
Quality API:    http://<server-ip>:5001
MySQL:          localhost:3306
```

**Database connection info:**
- Host: quality-mysql
- Port: 3306
- Database: github_hub
- Username: root
- Password: root123456

---

## Script Usage Workflow

### Development Workflow
```bash
# 1. Build everything
./prepare.sh

# 2. Run tests
./test.sh -v

# 3. Local deployment
./deploy.sh

# 4. Test webhooks
./test_webhook.sh

# 5. Load test
./loadtest.sh moderate
```

### Production Deployment Workflow
```bash
# On development machine:

# 1. Build everything
./prepare.sh

# 2. Save images to tar files
./save-images.sh

# 3. Transfer project directory to target server
rsync -avz /root/dev/github-hub-main/ root@<target-server>:/root/github-hub/

# On target server:

# 4. Start all services
cd /root/github-hub
./start.sh

# Or use recovery mode for fresh install
./start.sh -r
```

### Maintenance Workflow
```bash
# Stop services
./stop.sh

# Update code and rebuild
./prepare.sh

# Save new images
./save-images.sh

# Transfer and restart on target
./start.sh
```

---

### Private Registry Workflow

If you have access to a private Docker registry, you can use the registry-based deployment workflow instead of transferring tar files.

**Registry Configuration:**
- Registry: `acr.aishu.cn`
- Repository: `ghh`
- Images:
  - `acr.aishu.cn/ghh/ghh-server:latest`
  - `acr.aishu.cn/ghh/quality-server:latest`
  - `acr.aishu.cn/ghh/quality-frontend:latest`

#### Build and Push (Development Machine)

```bash
# Option 1: Build and push in one command
./build-push.sh

# Option 2: Separate steps
./prepare.sh          # Build images
./push-images.sh      # Push to registry (will prompt for password)
```

**push-images.sh options:**
```bash
./push-images.sh              # Push all images
./push-images.sh ghh-server   # Push specific image
```

#### Pull and Deploy (Target Server)

```bash
# Transfer pull-start.sh to target server
scp pull-start.sh root@<target-server>:/root/github-hub/

# On target server, pull images and start services
cd /root/github-hub
./pull-start.sh              # Pull and start (upgrade mode)
./pull-start.sh -r           # Pull and start (recover mode)
```

**Benefits of using private registry:**
- No need to transfer large tar files
- Faster deployment
- Better version control
- Easier rollback to previous versions

**Choosing between workflows:**

| Scenario | Recommended Workflow |
|----------|---------------------|
| Air-gapped network | Tar files (save-images.sh + start.sh) |
| Stable internet | Private registry (build-push.sh + pull-start.sh) |
| Multiple target servers | Private registry (push once, pull multiple times) |

---

# Development

## Project Structure

```
github-hub/
├── cmd/
│   ├── ghh/                   # ghh CLI client entry point
│   ├── ghh-server/            # ghh server entry point
│   └── quality-server/        # quality server entry point
├── internal/
│   ├── client/                # HTTP client for ghh
│   ├── server/                # ghh server handlers
│   ├── storage/               # Workspace storage
│   ├── config/                # Config loader
│   ├── version/               # Version string
│   └── quality/
│       ├── api/               # API handlers
│       ├── handlers/          # Event handlers
│       ├── models/            # Data models
│       ├── storage/           # Storage layer
│       └── data/              # Mock test data
├── loadtest/                  # Load testing tool
├── scripts/
│   └── init-mysql.sql         # Database initialization
├── frontend/                  # React + Vite frontend
├── bin/                       # Compiled binaries
├── test.sh                    # Unit test script
├── test_webhook.sh            # Webhook simulation script
├── loadtest.sh                # Load test script
├── prepare.sh                 # Prepare script
└── deploy.sh                  # Deploy script
```

## Tech Stack

- **Backend**: Go 1.24
- **Database**: MySQL
- **Frontend**: React + Vite + Nginx
- **Container**: Docker
- **Base Image**: Alpine Linux 3.19

---

# Testing

## Unit Tests

Run all tests using the `test.sh` script:

```bash
# Run all tests
./test.sh

# Verbose output
./test.sh -v

# Show coverage
./test.sh -c

# Race detection + coverage
./test.sh -r -c

# Test specific module
./test.sh ./internal/quality
```

## Test Coverage

- `internal/quality/models` - 94.1% coverage
  - Event creation and parsing
  - Quality check generation
  - Event filtering logic
- `internal/quality/handlers` - 96.7% coverage
  - Push event processing
  - Pull Request event processing
- `internal/quality/storage` - Mock storage implementation
  - CRUD operations
  - Error handling

## API Testing

### Using Web UI
1. Visit `http://localhost:8081`
2. Click "Mock测试" in the navigation
3. Select test type and submit

### Using curl

```bash
# Send test PR event
curl -X POST http://localhost:5001/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: pull_request" \
  -d @test-pr-event.json

# Simulate predefined event
curl -X POST http://localhost:5001/api/mock/simulate/push \
  -H "Content-Type: application/json"

# Execute custom test
curl -X POST http://localhost:5001/api/custom-test \
  -H "Content-Type: application/json" \
  -d '{"payload": {
    "event_type": "push",
    "repository": "owner/repo",
    "branch": "main",
    "commit_sha": "abc123",
    "pusher": "username"
  }}'
```

---

# Load Testing

Project includes a high-performance load testing tool for testing quality-server webhook handling.

## Quick Start

```bash
# First run requires compilation
./loadtest.sh

# Basic test (100 requests, 10 concurrent)
./loadtest.sh basic

# Moderate load (500 requests, 20 concurrent)
./loadtest.sh moderate

# Heavy load (1000 requests, 50 concurrent)
./loadtest.sh heavy

# Stress test (5000 requests, 100 concurrent)
./loadtest.sh stress

# Extreme test (10000 requests, 200 concurrent)
./loadtest.sh extreme

# PR event test
./loadtest.sh pr

# Rate-limited test (500 requests, 100 QPS)
./loadtest.sh rate
```

## Custom Tests

```bash
# Custom parameters
./loadtest.sh custom -n 1000 -c 50 -type push

# Rate-limited test
./loadtest.sh custom -n 500 -c 20 -qps 100

# Specify server
QUALITY_SERVER_URL=http://localhost:5001 ./loadtest.sh stress
```

## Performance Benchmarks

| Scenario | Requests | Concurrent | Throughput | Success Rate |
|----------|----------|------------|------------|--------------|
| Basic | 100 | 10 | ~1,300 req/s | 100% |
| Moderate | 500 | 20 | ~2,600 req/s | 100% |
| Heavy | 1,000 | 50 | ~6,500 req/s | 100% |
| Stress | 5,000 | 100 | ~4,800 req/s | 100% |
| Extreme | 10,000 | 200 | ~5,700 req/s | 100% |

## Operations

### Check Container Status

```bash
docker ps --filter "network=github-hub-network"
```

### View Logs

```bash
docker logs -f ghh-server
docker logs -f quality-server
docker logs -f quality-frontend
docker logs -f quality-mysql
```

### Stop Services

```bash
docker stop ghh-server quality-server quality-frontend quality-mysql
```

### Backup Data

```bash
# Backup MySQL
docker exec quality-mysql mysqldump -uroot -proot123456 github_hub > backup.sql

# Backup ghh-server cache
tar -czf ghh-cache-backup.tar.gz data/ghh-server/
```

### Restore Data

```bash
# Restore MySQL
docker exec -i quality-mysql mysql -uroot -proot123456 github_hub < backup.sql

# Restore ghh-server cache
tar -xzf ghh-cache-backup.tar.gz
```

## Troubleshooting

### Container Won't Start

1. Check port availability: `ss -tlnp | grep 8080`
2. Check existing containers: `docker ps -a | grep -E 'ghh-server|quality-server|quality-mysql'`
3. View container logs: `docker logs <container_name>`
4. Clean and redeploy: `docker rm -f $(docker ps -aq --filter "network=github-hub-network") && ./deploy.sh`

### Database Connection Failed

1. Check MySQL container: `docker ps | grep quality-mysql`
2. Check network: `docker exec quality-server ping quality-mysql`
3. Verify credentials (root/root123456)
4. Wait for MySQL: `docker logs quality-mysql | grep "ready for connections"`

### Webhook Events Not Processed

1. Check server logs: `docker logs quality-server`
2. Verify event matches filter rules (main branch only)
3. Verify GitHub Webhook configuration

## License

MIT License

## Contact

For questions or suggestions, please contact the project maintainers.
