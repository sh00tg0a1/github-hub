# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

github-hub is a Go application with two binaries: a server (`ghh-server`) that mirrors/caches GitHub repositories and packages, and a CLI client (`ghh`) that requests downloads and manages the server-side cache. Designed for environments without direct internet access.

## Build & Run Commands

```bash
# Build binaries
go build -o bin/ghh ./cmd/ghh
go build -o bin/ghh-server ./cmd/ghh-server

# Run client
go run ./cmd/ghh --help

# Run server (GITHUB_TOKEN optional but recommended for rate limits)
GITHUB_TOKEN=... bin/ghh-server --addr :8080 --root data

# Run tests with race detection and coverage
go test ./... -race -cover

# Code checks
go vet ./...
go fmt ./...
```

## Architecture

```
cmd/
├── ghh/main.go          # CLI client entry point
└── ghh-server/main.go   # HTTP server entry point

internal/
├── client/client.go     # HTTP client for ghh CLI → server communication
├── server/server.go     # HTTP handlers + janitor (cleanup goroutine)
├── storage/storage.go   # Workspace storage: downloads from GitHub, caches zips
├── config/config.go     # Client YAML/JSON config loader
└── version/version.go   # Version string (set via ldflags)
```

**Key flows:**
- **Download**: Client → `GET /api/v1/download?repo=...&branch=...` → Server checks cache → If missing, downloads from `codeload.github.com` → Streams zip back
- **Storage layout**: `<root>/users/<user>/repos/<owner>/<repo>/<branch>.zip` with `.meta` (SHA) and `.commit.txt` files
- **Package caching**: `<root>/users/<user>/packages/<url-hash>/<filename>` keyed by SHA256 of URL
- **Janitor**: Background goroutine runs every minute, deletes items idle >24h

**API endpoints** (in `internal/server/server.go`):
- `GET /api/v1/download` - download repo zip
- `GET /api/v1/download/commit` - get cached commit SHA
- `GET /api/v1/download/package` - download arbitrary URL with server-side caching
- `POST /api/v1/branch/switch` - ensure branch exists in cache
- `GET /api/v1/dir/list` - list directory contents
- `DELETE /api/v1/dir` - delete path from cache

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

## Testing

Test files: `*_test.go` alongside source. Key test files:
- `internal/server/server_test.go` - API handler tests with fake store
- `internal/server/server_download_test.go` - download-specific tests
- `internal/storage/storage_test.go` - storage layer tests
- `cmd/ghh/main_test.go` - CLI integration tests

Run single test: `go test -v -run TestName ./internal/server/`
