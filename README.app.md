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
