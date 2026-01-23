# github-hub Application Guide
Practical workflows for running `ghh-server`, using the `ghh` CLI, and browsing cached repositories.

## Goals
- Mirror GitHub repos into an offline-friendly cache.
- Reuse cached branches for repeated downloads to save bandwidth.
- Provide a simple web view for inspecting and cleaning the cache.

## Workflows
> Commands below use compiled `bin/ghh`; you can also use `go run ./cmd/ghh` instead.

- **Download repository** (client user/token override server defaults):  
  - Start server first (see deployment options below).  
  - `bin/ghh --server http://localhost:8080 --user alice --token <PAT> download --repo owner/repo --branch main --dest out.zip`  
  - Use `--extract` to unpack directly into a directory.
- **Sparse download** (download only specific directories - ideal for large repos):  
  - `bin/ghh --server http://localhost:8080 download-sparse --repo owner/repo --branch main --path src --dest out.zip`  
  - Multiple directories: `--path src --path docs --path configs`  
  - With extraction: `--dest ./project --extract`
- **Pre-cache branch** (have server download specified branch ahead of time for faster subsequent downloads):  
  - `bin/ghh --server http://localhost:8080 switch --repo owner/repo --branch dev`
- **Browse cache**:  
  - Open `http://localhost:8080/`, navigate under `users/<user>/repos/...`, entries are `<branch>.zip`, filter by name/path supported.
- **Clean up cache**:  
  - `bin/ghh --server http://localhost:8080 rm --path repos/owner/repo --r` (recursive delete, server auto-prefixes user path)  
  - Remove single file: omit `--r` or use `recursive=false`

## Quick Start

### Minimal commands (two lines to run)
```bash
# 1. Start server
go run ./cmd/ghh-server

# 2. Download repo (open new terminal)
go run ./cmd/ghh download --repo owner/repo --dest out.zip
```

### Full startup flow

1. **Start server** (no build needed):

```bash
# Minimal
go run ./cmd/ghh-server

# With options (Linux/macOS)
GITHUB_TOKEN=<optional> go run ./cmd/ghh-server --addr :8080 --root data

# With options (Windows PowerShell)
$env:GITHUB_TOKEN="<optional>"; go run ./cmd/ghh-server --addr :8080 --root data
```

2. **Open Web UI**: Visit `http://localhost:8080/` in your browser

3. **Use client to download** (open new terminal):

```bash
# Minimal
go run ./cmd/ghh download --repo owner/repo --dest out.zip

# With options
go run ./cmd/ghh --server http://localhost:8080 download --repo owner/repo --branch main --dest out.zip --extract
```

## Deployment options

### Native build

```bash
# Build
go build -o bin/ghh-server ./cmd/ghh-server
go build -o bin/ghh ./cmd/ghh

# Run server
GITHUB_TOKEN=<optional> bin/ghh-server --addr :8080 --root data
```

### Docker

```bash
# Build image
docker build -t ghh-server .

# Run (Linux/macOS)
docker run -p 8080:8080 -v $(pwd)/data:/data -e GITHUB_TOKEN=your_token ghh-server

# Run (Windows PowerShell)
docker run -p 8080:8080 -v ${PWD}/data:/data -e GITHUB_TOKEN=your_token ghh-server
```

### Make (recommended)

```bash
# Build
make build          # Build both server and client
make build-server   # Build server only
make build-client   # Build client only

# Build and run server (one command)
make run            # Build and run server on :8080

# Or with custom options
GITHUB_TOKEN=<token> SERVER_ADDR=:9090 SERVER_ROOT=./mydata make run

# After build, run manually
GITHUB_TOKEN=<optional> bin/ghh-server --addr :8080 --root data

# Other commands
make test           # Run tests with race detection
make vet            # Run go vet
make fmt            # Format code
make clean          # Remove bin/ directory
```

## Command-line Reference

### Server (ghh-server)

```
ghh-server [options]
```

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--addr` | - | `:8080` | Listen address |
| `--root` | - | `data` | Cache root directory |
| `--config` | - | - | Server config file path |
| `--version` | - | - | Print version and exit |
| - | `GITHUB_TOKEN` | - | GitHub API token (for private repos or higher rate limits) |

### Client (ghh)

```
ghh [global options] <command> [command options]
```

#### Global Options

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--server` | `GHH_BASE_URL` | `http://localhost:8080` | Server address |
| `--token` | `GHH_TOKEN` | - | Auth token |
| `--user` | `GHH_USER` | `default` | User name (for cache isolation) |
| `--config` | `GHH_CONFIG` | - | Client config file path |
| `--timeout` | - | `30s` | HTTP timeout |
| `--insecure` | - | `false` | Skip TLS certificate verification |
| `--version` | - | - | Print version and exit |

#### download Command

Download repository code (as zip or extracted).

```bash
ghh download --repo <owner/repo> --dest <path> [options]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--repo` | ✅ | Repository identifier (e.g. `owner/repo`) |
| `--dest` | ❌ | Destination path (see behavior below) |
| `--branch` | ❌ | Branch name (auto-detects default branch if empty) |
| `--extract` | ❌ | Extract to directory (saves as zip file if omitted) |

**Destination behavior**:
- Empty: saves to `./<repo>.zip`, extracts to `./` (with `--extract`)
- Existing directory: saves to `<dir>/<repo>.zip`, extracts to `<dir>/` (with `--extract`)
- File path: saves zip to that path, extracts to its parent directory (with `--extract`)

**Note**: The zip file is always saved. With `--extract`, contents are also extracted to the target directory.

#### download-sparse Command

Download only specific directories from a repository using `git archive`. Ideal for large repos where you only need a subset of the code.

```bash
ghh download-sparse --repo <owner/repo> --path <dir> [--path <dir2>] [--dest <path>] [options]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--repo` | ✅ | Repository identifier (e.g. `owner/repo`) |
| `--path` | ✅ | Directory to include (can specify multiple times) |
| `--dest` | ❌ | Destination path (default: `./<repo>-<branch>.zip`) |
| `--branch` | ❌ | Branch name (defaults to `main`) |
| `--extract` | ❌ | Extract to directory after download |

**Examples**:
```bash
# Download single directory (auto-named: repo-main.zip)
ghh download-sparse --repo owner/repo --path src

# Download from specific branch (auto-named: repo-release-0.2.0.zip)
ghh download-sparse --repo owner/repo --branch release/0.2.0 --path src

# Download multiple directories with explicit name
ghh download-sparse --repo owner/repo --path src --path docs --dest output.zip

# Download and extract to directory
ghh download-sparse --repo owner/repo --path src --dest ./project --extract
```

**Notes**:
- Default filename includes sanitized branch name (e.g., `release/0.2.0` → `release-0.2.0`)
- Sparse download uses a shared bare Git cache on the server (`git-cache/<owner>/<repo>.git`), enabling incremental updates via `git fetch`
- Uses `git archive` for fast direct export from bare repository

#### switch Command

Pre-cache a specific branch (have server download ahead of time).

```bash
ghh switch --repo <owner/repo> --branch <branch>
```

| Flag | Required | Description |
|------|----------|-------------|
| `--repo` | ✅ | Repository identifier |
| `--branch` | ✅ | Branch name |

#### ls Command

List server cache directory.

```bash
ghh ls [--path <path>] [--raw]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--path` | ❌ | Remote path (default `.`) |
| `--raw` | ❌ | Output raw JSON |

#### rm Command

Delete server cache.

```bash
ghh rm --path <path> [-r]
```

| Flag | Required | Description |
|------|----------|-------------|
| `--path` | ✅ | Remote path |
| `-r` | ❌ | Recursive delete |

## HTTP API Reference

You can also access the server directly via HTTP without the `ghh` client.

### Download

```bash
# GET /api/v1/download
# Params: repo (required), branch (optional), user (optional)

# Basic usage - download default branch
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo"

# With specific branch
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo&branch=main"

# With user isolation (for cache grouping)
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo&branch=main&user=alice"

# With custom token (for private repos)
curl -H "Authorization: Bearer ghp_xxxx" \
     -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/private-repo"

# Windows PowerShell
Invoke-WebRequest -Uri "http://localhost:8080/api/v1/download?repo=owner/repo" -OutFile repo.zip
```

| Param/Header | Required | Description |
|--------------|----------|-------------|
| `repo` | ✅ | Repository identifier, format `owner/repo` |
| `branch` | ❌ | Branch name, auto-detects default if empty |
| `user` | ❌ | User name (can also use `X-GHH-User` header) |
| `Authorization` | ❌ | Format `Bearer <token>`, for private repos |

### Sparse Download

```bash
# GET /api/v1/download/sparse
# Params: repo (required), paths (required, comma-separated), branch (optional)

# Download single directory
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src"

# Download multiple directories
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src,docs,configs"

# With specific branch
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&branch=develop&paths=src"

# Windows PowerShell
Invoke-WebRequest -Uri "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src" -OutFile sparse.zip
```

| Param/Header | Required | Description |
|--------------|----------|-------------|
| `repo` | ✅ | Repository identifier, format `owner/repo` |
| `paths` | ✅ | Comma-separated list of directories to include |
| `branch` | ❌ | Branch name (defaults to `main`) |
| `Authorization` | ❌ | Format `Bearer <token>`, for private repos |

**Response headers**:
- `X-GHH-Commit`: Short commit SHA of the downloaded content

### Branch Switch

```bash
# POST /api/v1/branch/switch
# Body: JSON {"repo": "owner/repo", "branch": "branch"}

curl -X POST "http://localhost:8080/api/v1/branch/switch" \
     -H "Content-Type: application/json" \
     -d '{"repo": "owner/repo", "branch": "dev"}'

# With user and token
curl -X POST "http://localhost:8080/api/v1/branch/switch" \
     -H "Content-Type: application/json" \
     -H "X-GHH-User: alice" \
     -H "Authorization: Bearer ghp_xxxx" \
     -d '{"repo": "owner/repo", "branch": "feature"}'

# Windows PowerShell
Invoke-RestMethod -Uri "http://localhost:8080/api/v1/branch/switch" `
    -Method Post -ContentType "application/json" `
    -Body '{"repo": "owner/repo", "branch": "dev"}'
```

### List Directory

```bash
# GET /api/v1/dir/list
# Params: path (optional, defaults to .)

# List root directory
curl "http://localhost:8080/api/v1/dir/list"

# List specific path
curl "http://localhost:8080/api/v1/dir/list?path=repos/owner/repo"
```

### Delete

```bash
# DELETE /api/v1/dir
# Params: path (required), recursive (optional, defaults to false)

# Delete single file
curl -X DELETE "http://localhost:8080/api/v1/dir?path=repos/owner/repo/main.zip"

# Delete directory recursively
curl -X DELETE "http://localhost:8080/api/v1/dir?path=repos/owner/repo&recursive=true"
```

## Paths and configuration

- Cache layout: `data/users/<user>/repos/<owner>/<repo>/<branch>.zip` (archives only, no extraction on disk); control root with `--root` or server config.
- Base URL: `--server` flag or `GHH_BASE_URL`.  
- User name: `--user` flag or `GHH_USER` (defaults to server `default_user` when empty).
- Auth token: `--token` or `GHH_TOKEN` (client); server fallback token via config or `GITHUB_TOKEN`.  
- Custom API paths: override per-flag (`--api-*`) or via config file (`configs/config.yaml` from `configs/config.example.yaml`).
- Cleanup: server janitor runs every minute and removes repos idle >24h.

## Related docs
- English overview: `README.md`
- 中文文档：`README.zh.md`
- 应用指南（中文）：`README.app.zh.md`
