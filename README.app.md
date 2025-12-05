# github-hub Application Guide
Practical workflows for running `ghh-server`, using the `ghh` CLI, and browsing cached repositories.

## Goals
- Mirror GitHub repos into an offline-friendly cache.
- Reuse cached branches for repeated downloads to save bandwidth.
- Provide a simple web view for inspecting and cleaning the cache.

## Workflows
- Cache a repository (client user/token override server defaults):  
  - Start server (native or Docker).  
  - `bin/ghh --server http://localhost:8080 --user alice --token <PAT> download --repo owner/repo --branch main --dest out.zip`  
  - Use `--extract` to unpack directly into a directory.
- Switch branch on the server:  
  - `bin/ghh --server http://localhost:8080 switch --repo owner/repo --branch dev`
- Browse cached files:  
  - Open `http://localhost:8080/`, navigate under `users/<user>/repos/...` (default user when header missing), filter by name/path, and download via the API if needed.
- Clean up cache:  
  - `bin/ghh --server http://localhost:8080 rm --path repos/owner/repo --r` for recursive delete (server will prefix the current user).  
  - Individual files can be removed with `recursive=false`.

## Deployment options
- Native: `go build -o bin/ghh-server ./cmd/ghh-server && GITHUB_TOKEN=<optional> bin/ghh-server --addr :8080 --root data`
- Docker:  
  - Build: `docker build -t ghh-server .`  
  - Run (Windows): `docker run -p 8080:8080 -v %CD%\\data:/data -e GITHUB_TOKEN=your_token ghh-server`  
  - Run (Linux/macOS): `docker run -p 8080:8080 -v $(pwd)/data:/data -e GITHUB_TOKEN=your_token ghh-server`

## Paths and configuration
- Cache layout: `data/users/<user>/repos/<owner>/<repo>/<branch>`; control root with `--root` or server config.
- Base URL: `--server` flag or `GHH_BASE_URL`.  
- User name: `--user` flag or `GHH_USER` (defaults to server `default_user` when empty).
- Auth token: `--token` or `GHH_TOKEN` (client); server fallback token via config or `GITHUB_TOKEN`.  
- Custom API paths: override per-flag (`--api-*`) or via config file (`configs/config.yaml` from `configs/config.example.yaml`).
- Cleanup: server janitor runs every minute and removes repos idle >24h.

## Related docs
- English overview: `README.md`
- 中文文档：`README.zh.md`
- 应用指南（中文）：`README.app.zh.md`
