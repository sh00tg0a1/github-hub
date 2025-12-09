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

