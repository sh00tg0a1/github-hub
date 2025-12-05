# github-hub 应用指南
运行 `ghh-server`、使用 `ghh` CLI 以及浏览缓存仓库的实用工作流程。

## 目标
- 将 GitHub 仓库镜像到离线友好的缓存中。
- 重复下载时复用缓存分支以节省带宽。
- 提供简单的 Web 界面用于检查和清理缓存。

## 工作流程
- 缓存仓库（客户端用户/token 可覆盖服务端默认值）：  
  - 启动服务器（原生或 Docker）。  
  - `bin/ghh --server http://localhost:8080 --user alice --token <PAT> download --repo owner/repo --branch main --dest out.zip`  
  - 使用 `--extract` 直接解压到目录。
- 在服务端切换分支：  
  - `bin/ghh --server http://localhost:8080 switch --repo owner/repo --branch dev`
- 浏览缓存文件：  
  - 打开 `http://localhost:8080/`，导航到 `users/<user>/repos/...`（当缺少 header 时使用默认用户），按名称/路径过滤，需要时通过 API 下载。
- 清理缓存：  
  - `bin/ghh --server http://localhost:8080 rm --path repos/owner/repo --r` 用于递归删除（服务端会自动添加当前用户前缀）。  
  - 单个文件可通过 `recursive=false` 删除。

## 部署选项
- 原生：`go build -o bin/ghh-server ./cmd/ghh-server && GITHUB_TOKEN=<可选> bin/ghh-server --addr :8080 --root data`
- Docker：  
  - 构建：`docker build -t ghh-server .`  
  - 运行（Windows）：`docker run -p 8080:8080 -v %CD%\\data:/data -e GITHUB_TOKEN=your_token ghh-server`  
  - 运行（Linux/macOS）：`docker run -p 8080:8080 -v $(pwd)/data:/data -e GITHUB_TOKEN=your_token ghh-server`

## 路径和配置
- 缓存布局：`data/users/<user>/repos/<owner>/<repo>/<branch>`；通过 `--root` 或服务端配置控制根目录。
- 基础 URL：`--server` 标志或 `GHH_BASE_URL`。  
- 用户名：`--user` 标志或 `GHH_USER`（为空时默认为服务端 `default_user`）。
- 认证 token：`--token` 或 `GHH_TOKEN`（客户端）；服务端回退 token 通过配置或 `GITHUB_TOKEN`。  
- 自定义 API 路径：通过每个标志（`--api-*`）或配置文件（从 `configs/config.example.yaml` 复制为 `configs/config.yaml`）覆盖。
- 清理：服务端 janitor 每分钟运行一次，删除空闲超过 24 小时的仓库。

## 相关文档
- 英文概览：`README.md`
- 中文文档：`README.zh.md`
- 应用指南（英文）：`README.app.md`

