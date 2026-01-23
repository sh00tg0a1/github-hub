# github-hub 应用指南
运行 `ghh-server`、使用 `ghh` CLI 以及浏览缓存仓库的实用工作流程。

## 目标
- 将 GitHub 仓库镜像到离线友好的缓存中。
- 重复下载时复用缓存分支以节省带宽。
- 提供简单的 Web 界面用于检查和清理缓存。

## 工作流程
> 以下命令使用编译后的 `bin/ghh`，也可用 `go run ./cmd/ghh` 替代。

- **下载仓库**（客户端用户/token 可覆盖服务端默认值）：  
  - 先启动服务端（见下方部署选项）。  
  - `bin/ghh --server http://localhost:8080 --user alice --token <PAT> download --repo owner/repo --branch main --dest out.zip`  
  - 使用 `--extract` 直接解压到目录。
- **稀疏下载**（仅下载指定目录 - 适用于大型仓库）：  
  - `bin/ghh --server http://localhost:8080 download-sparse --repo owner/repo --branch main --path src --dest out.zip`  
  - 多个目录：`--path src --path docs --path configs`  
  - 带解压：`--dest ./project --extract`
- **预缓存分支**（让服务端提前下载指定分支，后续下载更快）：  
  - `bin/ghh --server http://localhost:8080 switch --repo owner/repo --branch dev`
- **浏览缓存**：  
  - 打开 `http://localhost:8080/`，导航到 `users/<user>/repos/...`，条目为 `<branch>.zip`，支持按名称/路径过滤。
- **清理缓存**：  
  - `bin/ghh --server http://localhost:8080 rm --path repos/owner/repo --r`（递归删除，服务端自动添加用户前缀）  
  - 删除单个文件：不带 `--r` 或 `recursive=false`

## 快速启动

### 最简命令（两行即可运行）
```bash
# 1. 启动服务端
go run ./cmd/ghh-server

# 2. 下载仓库（新开终端）
go run ./cmd/ghh download --repo owner/repo --dest out.zip
```

### 完整启动流程

1. **启动服务端**（无需编译）：

```bash
# 最简
go run ./cmd/ghh-server

# 带参数（Linux/macOS）
GITHUB_TOKEN=<可选> go run ./cmd/ghh-server --addr :8080 --root data

# 带参数（Windows PowerShell）
$env:GITHUB_TOKEN="<可选>"; go run ./cmd/ghh-server --addr :8080 --root data
```

2. **打开 Web UI**：浏览器访问 `http://localhost:8080/`

3. **使用客户端下载**（新开终端）：

```bash
# 最简
go run ./cmd/ghh download --repo owner/repo --dest out.zip

# 带参数
go run ./cmd/ghh --server http://localhost:8080 download --repo owner/repo --branch main --dest out.zip --extract
```

## 部署选项

### 原生编译

```bash
# 编译
go build -o bin/ghh-server ./cmd/ghh-server
go build -o bin/ghh ./cmd/ghh

# 运行服务端
GITHUB_TOKEN=<可选> bin/ghh-server --addr :8080 --root data
```

### Docker

```bash
# 构建镜像
docker build -t ghh-server .

# 运行（Linux/macOS）
docker run -p 8080:8080 -v $(pwd)/data:/data -e GITHUB_TOKEN=your_token ghh-server

# 运行（Windows PowerShell）
docker run -p 8080:8080 -v ${PWD}/data:/data -e GITHUB_TOKEN=your_token ghh-server
```

### Make（推荐）

```bash
# 编译
make build          # 编译服务端和客户端
make build-server   # 仅编译服务端
make build-client   # 仅编译客户端

# 编译并运行服务端（一条命令）
make run            # 编译并在 :8080 运行服务端

# 或使用自定义选项
GITHUB_TOKEN=<token> SERVER_ADDR=:9090 SERVER_ROOT=./mydata make run

# 编译后手动运行
GITHUB_TOKEN=<可选> bin/ghh-server --addr :8080 --root data

# 其他命令
make test           # 运行测试（带竞态检测）
make vet            # 运行 go vet
make fmt            # 格式化代码
make clean          # 清理 bin/ 目录
```

## 命令行参数

### 服务端 (ghh-server)

```
ghh-server [选项]
```

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `--addr` | - | `:8080` | 监听地址 |
| `--root` | - | `data` | 缓存根目录 |
| `--config` | - | - | 服务端配置文件路径 |
| - | `GITHUB_TOKEN` | - | GitHub API token（用于私有仓库或提高速率限制） |

### 客户端 (ghh)

```
ghh [全局选项] <命令> [命令选项]
```

#### 全局选项

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `--server` | `GHH_BASE_URL` | `http://localhost:8080` | 服务端地址 |
| `--token` | `GHH_TOKEN` | - | 认证 token |
| `--user` | `GHH_USER` | `default` | 用户名（用于缓存隔离） |
| `--config` | `GHH_CONFIG` | - | 客户端配置文件路径 |
| `--timeout` | - | `30s` | HTTP 超时时间 |
| `--insecure` | - | `false` | 跳过 TLS 证书验证 |

#### download 命令

下载仓库代码（zip 或解压）。

```bash
ghh download --repo <owner/repo> --dest <路径> [选项]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--repo` | ✅ | 仓库标识（如 `owner/repo`） |
| `--dest` | ❌ | 目标路径（见下方说明） |
| `--branch` | ❌ | 分支名（为空时自动获取默认分支） |
| `--extract` | ❌ | 解压到目录（不加则保存为 zip 文件） |

**目标路径行为**：
- 留空：保存为 `./<repo>.zip`，解压到 `./`（带 `--extract`）
- 已存在的目录：保存为 `<dir>/<repo>.zip`，解压到 `<dir>/`（带 `--extract`）
- 文件路径：保存 zip 到该路径，解压到其父目录（带 `--extract`）

**注意**：zip 文件始终会保存。带 `--extract` 时，内容会同时解压到目标目录。

#### download-sparse 命令

使用 `git archive` 仅下载仓库中的指定目录。适用于大型仓库只需要部分代码的场景。

```bash
ghh download-sparse --repo <owner/repo> --path <目录> [--path <目录2>] [--dest <路径>] [选项]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--repo` | ✅ | 仓库标识（如 `owner/repo`） |
| `--path` | ✅ | 要包含的目录（可多次指定） |
| `--dest` | ❌ | 目标路径（默认：`./<repo>-<branch>.zip`） |
| `--branch` | ❌ | 分支名（默认 `main`） |
| `--extract` | ❌ | 下载后解压到目录 |

**示例**：
```bash
# 下载单个目录（自动命名：repo-main.zip）
ghh download-sparse --repo owner/repo --path src

# 下载指定分支（自动命名：repo-release-0.2.0.zip）
ghh download-sparse --repo owner/repo --branch release/0.2.0 --path src

# 下载多个目录并指定文件名
ghh download-sparse --repo owner/repo --path src --path docs --dest output.zip

# 下载并解压到目录
ghh download-sparse --repo owner/repo --path src --dest ./project --extract
```

**说明**：
- 默认文件名包含清理后的分支名（如 `release/0.2.0` → `release-0.2.0`）
- 稀疏下载使用服务端的共享 bare Git 缓存（`git-cache/<owner>/<repo>.git`），通过 `git fetch` 实现增量更新
- 使用 `git archive` 从 bare 仓库直接快速导出

#### switch 命令

预缓存指定分支（让服务端提前下载）。

```bash
ghh switch --repo <owner/repo> --branch <分支名>
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--repo` | ✅ | 仓库标识 |
| `--branch` | ✅ | 分支名 |

#### ls 命令

列出服务端缓存目录。

```bash
ghh ls [--path <路径>] [--raw]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--path` | ❌ | 远程路径（默认 `.`） |
| `--raw` | ❌ | 输出原始 JSON |

#### rm 命令

删除服务端缓存。

```bash
ghh rm --path <路径> [-r]
```

| 参数 | 必填 | 说明 |
|------|------|------|
| `--path` | ✅ | 远程路径 |
| `-r` | ❌ | 递归删除目录 |

## HTTP API 参考

除了使用 `ghh` 客户端，你也可以直接通过 HTTP 接口访问服务器。

### 下载仓库

```bash
# GET /api/v1/download
# 参数: repo (必需), branch (可选), user (可选)

# 基本用法 - 下载仓库默认分支
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo"

# 指定分支
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo&branch=main"

# 带用户隔离（用于缓存分组）
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo&branch=main&user=alice"

# 使用自定义 Token（访问私有仓库）
curl -H "Authorization: Bearer ghp_xxxx" \
     -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/private-repo"

# Windows PowerShell
Invoke-WebRequest -Uri "http://localhost:8080/api/v1/download?repo=owner/repo" -OutFile repo.zip
```

| 参数/Header | 必需 | 说明 |
|-------------|------|------|
| `repo` | ✅ | 仓库标识，格式 `owner/repo` |
| `branch` | ❌ | 分支名，留空则自动获取默认分支 |
| `user` | ❌ | 用户名（也可通过 `X-GHH-User` header 传递） |
| `Authorization` | ❌ | 格式 `Bearer <token>`，用于私有仓库 |

### 稀疏下载

```bash
# GET /api/v1/download/sparse
# 参数: repo (必需), paths (必需，逗号分隔), branch (可选)

# 下载单个目录
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src"

# 下载多个目录
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src,docs,configs"

# 指定分支
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&branch=develop&paths=src"

# Windows PowerShell
Invoke-WebRequest -Uri "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src" -OutFile sparse.zip
```

| 参数/Header | 必需 | 说明 |
|-------------|------|------|
| `repo` | ✅ | 仓库标识，格式 `owner/repo` |
| `paths` | ✅ | 逗号分隔的目录列表 |
| `branch` | ❌ | 分支名（默认 `main`） |
| `Authorization` | ❌ | 格式 `Bearer <token>`，用于私有仓库 |

**响应头**：
- `X-GHH-Commit`：下载内容的短提交 SHA

### 预缓存分支

```bash
# POST /api/v1/branch/switch
# Body: JSON {"repo": "owner/repo", "branch": "branch"}

curl -X POST "http://localhost:8080/api/v1/branch/switch" \
     -H "Content-Type: application/json" \
     -d '{"repo": "owner/repo", "branch": "dev"}'

# 带用户和 Token
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

### 列出缓存

```bash
# GET /api/v1/dir/list
# 参数: path (可选，默认 .)

# 列出根目录
curl "http://localhost:8080/api/v1/dir/list"

# 列出指定路径
curl "http://localhost:8080/api/v1/dir/list?path=repos/owner/repo"
```

### 删除缓存

```bash
# DELETE /api/v1/dir
# 参数: path (必需), recursive (可选，默认 false)

# 删除单个文件
curl -X DELETE "http://localhost:8080/api/v1/dir?path=repos/owner/repo/main.zip"

# 递归删除目录
curl -X DELETE "http://localhost:8080/api/v1/dir?path=repos/owner/repo&recursive=true"
```

## 路径和配置

- 缓存布局：`data/users/<user>/repos/<owner>/<repo>/<branch>.zip`（仅存储 zip 文件，不解压到磁盘）；通过 `--root` 或服务端配置控制根目录。
- 基础 URL：`--server` 标志或 `GHH_BASE_URL`。  
- 用户名：`--user` 标志或 `GHH_USER`（为空时默认为服务端 `default_user`）。
- 认证 token：`--token` 或 `GHH_TOKEN`（客户端）；服务端回退 token 通过配置或 `GITHUB_TOKEN`。  
- 自定义 API 路径：通过每个标志（`--api-*`）或配置文件（从 `configs/config.example.yaml` 复制为 `configs/config.yaml`）覆盖。
- 清理：服务端 janitor 每分钟运行一次，删除空闲超过 24 小时的仓库。

## 相关文档
- 英文概览：`README.md`
- 中文文档：`README.zh.md`
- 应用指南（英文）：`README.app.md`

