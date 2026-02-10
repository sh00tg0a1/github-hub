# GitHub-Hub

面向离线环境的综合 GitHub 工具集，包含两个主要组件：

1. **ghh-server** - GitHub 仓库缓存服务器，支持离线下载
2. **quality-server** - GitHub Webhook 质量检查服务，用于自动化 CI/CD

---

## 目录

- [ghh-server: GitHub 缓存服务器](#ghh-server-github-缓存服务器)
  - [功能特性](#功能特性)
  - [快速开始](#快速开始-1)
  - [部署方式](#部署方式)
  - [命令行参考](#命令行参考)
  - [HTTP API](#http-api)
- [quality-server: 质量检查服务](#quality-server-质量检查服务)
  - [功能特性](#功能特性-1)
  - [系统架构](#系统架构)
  - [快速开始](#快速开始-2)
  - [API 参考](#api-参考)
- [开发指南](#开发指南)
- [测试](#测试)
- [负载测试](#负载测试)

---

# ghh-server: GitHub 缓存服务器

轻量级 HTTP 服务器和命令行客户端，将 GitHub 仓库镜像到离线友好的缓存中。

## 功能特性

- **离线缓存**：下载并缓存 GitHub 仓库为 ZIP 文件
- **稀疏检出**：仅下载大型仓库中的特定目录
- **Git 模式**：使用裸 Git 仓库和 `git archive` 实现更快的下载和缓存复用
- **Web UI**：浏览和管理缓存仓库
- **用户隔离**：支持多用户，分离的缓存命名空间
- **自动清理**：自动删除空闲超过 24 小时的条目

## 快速开始

```bash
# 1. 启动服务端
go run ./cmd/ghh-server

# 2. 下载仓库（新开终端）
go run ./cmd/ghh download --repo owner/repo --dest out.zip
```

### 部署方式

**原生编译**
```bash
go build -o bin/ghh-server ./cmd/ghh-server
go build -o bin/ghh ./cmd/ghh
GITHUB_TOKEN=<可选> bin/ghh-server --addr :8080 --root data
```

**Docker**
```bash
docker build -t ghh-server .
docker run -p 8080:8080 -v $(pwd)/data:/data -e GITHUB_TOKEN=your_token ghh-server
```

**Make（推荐）**
```bash
make build   # 编译服务端和客户端
make run     # 编译并在 :8080 运行服务端
```

## 命令行参考

### 服务端 (ghh-server)

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `--addr` | - | `:8080` | 监听地址 |
| `--root` | - | `data` | 缓存根目录 |
| `--config` | - | - | 服务端配置文件路径 |
| - | `GITHUB_TOKEN` | - | GitHub API token |

### 客户端 (ghh)

#### 全局选项

| 参数 | 环境变量 | 默认值 | 说明 |
|------|---------|--------|------|
| `--server` | `GHH_BASE_URL` | `http://localhost:8080` | 服务端地址 |
| `--token` | `GHH_TOKEN` | - | 认证 token |
| `--user` | `GHH_USER` | `default` | 用户名 |

#### 命令

**download** - 下载仓库（ZIP 或解压）
```bash
ghh download --repo <owner/repo> --dest <路径> [选项]
```

| 参数 | 说明 |
|------|------|
| `--repo` | 仓库标识（必需） |
| `--branch` | 分支名（默认：main） |
| `--dest` | 目标路径 |
| `--extract` | 解压到目录 |
| `--legacy` | 使用旧的 GitHub API 而不是 git archive |

**download-sparse** - 仅下载指定目录
```bash
ghh download-sparse --repo <owner/repo> [--path <目录>] [--dest <路径>]
```

| 参数 | 说明 |
|------|------|
| `--repo` | 仓库标识（必需） |
| `--path` | 要包含的目录（可多次指定） |
| `--branch` | 分支名（默认：main） |
| `--extract` | 解压到目录 |

**switch** - 预缓存分支
```bash
ghh switch --repo <owner/repo> --branch <分支名>
```

**ls** - 列出服务端缓存
```bash
ghh ls [--path <路径>]
```

**rm** - 删除缓存
```bash
ghh rm --path <路径> [-r]
```

## HTTP API

### 下载仓库

```bash
# GET /api/v1/download
curl -o repo.zip "http://localhost:8080/api/v1/download?repo=owner/repo&branch=main"
```

| 参数 | 必需 | 说明 |
|------|------|------|
| `repo` | ✅ | 仓库标识（`owner/repo`） |
| `branch` | ❌ | 分支名 |
| `user` | ❌ | 用户名 |

### 稀疏下载

```bash
# GET /api/v1/download/sparse
curl -o sparse.zip "http://localhost:8080/api/v1/download/sparse?repo=owner/repo&paths=src,docs"
```

| 参数 | 必需 | 说明 |
|------|------|------|
| `repo` | ✅ | 仓库标识 |
| `paths` | ✅ | 逗号分隔的目录列表 |
| `branch` | ❌ | 分支名（默认：main） |

### 切换分支

```bash
# POST /api/v1/branch/switch
curl -X POST "http://localhost:8080/api/v1/branch/switch" \
  -H "Content-Type: application/json" \
  -d '{"repo": "owner/repo", "branch": "dev"}'
```

### 列出目录

```bash
# GET /api/v1/dir/list
curl "http://localhost:8080/api/v1/dir/list?path=repos/owner/repo"
```

### 删除

```bash
# DELETE /api/v1/dir
curl -X DELETE "http://localhost:8080/api/v1/dir?path=repos/owner/repo&recursive=true"
```

---

# quality-server: 质量检查服务

GitHub Webhook 质量检查服务，用于监控和处理 GitHub Pull Request 和 Push 事件，自动执行质量检查流程。

## 功能特性

- **GitHub Webhook 集成**：接收和处理 GitHub PR 和 Push 事件
- **智能事件过滤**：只处理合并到 main 分支的 PR 和 main 分支的 Push
- **多阶段质量检查**：支持基础 CI、部署、专项测试三个阶段
- **MySQL 持久化**：存储事件和质量检查数据
- **RESTful API**：完整的 API 接口用于查询和管理数据
- **Docker 支持**：容器化部署
- **Mock 测试**：支持预定义和自定义测试

## 系统架构

### 容器架构

项目包含 4 个 Docker 容器：

| 容器 | 端口 | 说明 |
|------|------|------|
| **ghh-server** | 8080 | GitHub 仓库缓存服务器 |
| **quality-server** | 5001 | GitHub webhook 质量引擎 |
| **quality-frontend** | 8081 | Web UI |
| **quality-mysql** | 3307 | MySQL 数据库 |

### 质量检查阶段

1. **基础 CI**
   - 编译检查
   - 代码规范检查
   - 安全扫描
   - 单元测试

2. **部署**
   - 部署检查

3. **专项测试**
   - API 测试
   - 模块端到端测试
   - 代理端到端测试
   - AI 端到端测试

## 快速开始

```bash
# 1. 准备 - 加载镜像、编译二进制、构建 Docker 镜像
./prepare.sh

# 2. 部署 - 启动所有 4 个容器
./deploy.sh
```

这将启动：
- **ghh-server** (端口 8080) - GitHub 缓存服务器
- **quality-server** (端口 5001) - 质量引擎 API
- **quality-frontend** (端口 8081) - Web UI
- **quality-mysql** (端口 3307) - 数据库

**MySQL 连接信息**：
- 数据库：`github_hub`
- 用户名：`root`
- 密码：`root123456`

## API 参考

### Webhook 端点

| 方法 | 端点 | 说明 |
|------|------|------|
| `POST` | `/webhook` | 接收 GitHub Webhook 事件 |

### 事件管理

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/events` | 获取事件列表 |
| `GET` | `/api/events/:id` | 获取事件详情 |
| `DELETE` | `/api/events` | 删除所有事件 |

### 质量检查

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/events/:eventID/quality-checks` | 获取质量检查列表 |
| `PUT` | `/api/quality-checks/:id` | 更新质量检查状态 |

### Mock 测试端点

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/mock/events` | 获取 Mock 事件模板 |
| `POST` | `/api/mock/simulate/:event-type` | 模拟预定义事件 |
| `POST` | `/api/custom-test` | 执行自定义测试 |

### 其他端点

| 方法 | 端点 | 说明 |
|------|------|------|
| `GET` | `/api/repositories` | 获取仓库列表 |
| `GET` | `/api/status` | 获取系统状态 |
| `POST` | `/api/login` | 用户登录 |
| `POST` | `/api/logout` | 用户登出 |
| `GET` | `/api/check-login` | 检查登录状态 |

## 事件过滤规则

### Push 事件
只处理 main 分支的 Push 事件，其他分支的 Push 事件会被忽略。

### Pull Request 事件
只处理从非 main 分支合并到 main 分支的 Pull Request 事件，其他 PR 事件会被忽略。

## 数据库结构

### github_events 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INT | 主键 |
| event_id | VARCHAR(36) | 事件唯一标识 |
| event_type | VARCHAR(50) | 事件类型 |
| event_status | VARCHAR(50) | 事件状态 |
| repository | VARCHAR(255) | 仓库名称 |
| branch | VARCHAR(255) | 分支名称 |
| target_branch | VARCHAR(255) | 目标分支 |
| commit_sha | VARCHAR(255) | 提交 SHA |
| pr_number | INT | PR 编号 |
| action | VARCHAR(50) | 操作类型 |
| pusher | VARCHAR(255) | 推送者 |
| author | VARCHAR(255) | 作者 |
| payload | JSON | 事件载荷 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |
| processed_at | TIMESTAMP | 处理时间 |

### pr_quality_checks 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INT | 主键 |
| github_event_id | VARCHAR(36) | 关联的事件 ID |
| check_type | VARCHAR(50) | 检查类型 |
| check_status | VARCHAR(50) | 检查状态 |
| stage | VARCHAR(50) | 检查阶段 |
| stage_order | INT | 阶段顺序 |
| check_order | INT | 检查顺序 |
| started_at | TIMESTAMP | 开始时间 |
| completed_at | TIMESTAMP | 完成时间 |
| duration_seconds | DOUBLE | 持续时间（秒） |
| error_message | TEXT | 错误信息 |
| output | TEXT | 输出 |
| retry_count | INT | 重试次数 |
| created_at | TIMESTAMP | 创建时间 |
| updated_at | TIMESTAMP | 更新时间 |

---

# Shell 脚本参考

本项目包含多个 Shell 脚本，用于构建、测试、部署和管理应用。

## 开发脚本

### prepare.sh
编译所有组件并准备 Docker 镜像。

```bash
./prepare.sh
```

**功能说明：**
1. 加载基础 Docker 镜像（alpine、golang、nginx、mysql）
2. 编译 Go 二进制文件（ghh-server、ghh、quality-server）
3. 构建 React 前端
4. 构建所有服务的 Docker 镜像

**输出结果：**
- `bin/ghh-server` - ghh-server 二进制文件
- `bin/ghh` - ghh CLI 二进制文件
- `bin/quality-server` - quality-server 二进制文件
- Docker 镜像：ghh-server:latest、quality-server:latest、quality-frontend:latest

---

### test.sh
运行所有 Go 模块的单元测试。

```bash
# 运行所有测试
./test.sh

# 详细输出（显示每个测试命令）
./test.sh -v

# 显示覆盖率报告
./test.sh -c

# 启用竞态检测
./test.sh -r

# 测试指定模块
./test.sh ./internal/quality

# 组合选项
./test.sh -v -r -c
```

**功能说明：**
- 对所有 Go 包运行 `go test`
- 支持详细模式显示测试命令
- 可选计算代码覆盖率
- 可选启用竞态检测
- 测试 8 个模块：cmd/ghh、cmd/ghh-server、internal/client、internal/server、internal/storage、internal/quality/api、internal/quality/handlers、internal/quality/storage

**测试结果示例：**
```
✓ cmd/ghh                  PASS
✓ cmd/ghh-server           PASS
✓ internal/client          PASS
✓ internal/server          PASS
✓ internal/storage         PASS
✓ internal/quality/api     PASS
✓ internal/quality/handlers PASS
✓ internal/quality/storage PASS

所有测试通过！ (8/8 模块)
```

---

### loadtest.sh
quality-server webhook 处理的负载测试工具。

```bash
# 基础测试（100 个请求，10 并发）
./loadtest.sh basic

# 中等负载（500 个请求，20 并发）
./loadtest.sh moderate

# 重负载（1000 个请求，50 并发）
./loadtest.sh heavy

# 压力测试（5000 个请求，100 并发）
./loadtest.sh stress

# 极限测试（10000 个请求，200 并发）
./loadtest.sh extreme

# PR 事件测试
./loadtest.sh pr

# 限速测试（500 个请求，100 QPS）
./loadtest.sh rate

# 自定义参数
./loadtest.sh custom -n 1000 -c 50 -type push

# 指定服务器
QUALITY_SERVER_URL=http://localhost:5001 ./loadtest.sh stress
```

**功能说明：**
- 首次运行时编译负载测试工具
- 向 quality-server 发送并发 webhook 请求
- 测量吞吐量、延迟和成功率
- 支持 push 和 PR 事件类型

---

### test_webhook.sh
模拟 GitHub webhook 事件用于测试 quality-server。

```bash
# 运行所有测试（默认服务器：http://10.4.174.125:5001）
./test_webhook.sh

# 指定自定义服务器
QUALITY_SERVER_URL=http://localhost:5001 ./test_webhook.sh
```

**功能说明：**
- 发送 Push 事件（main 分支）
- 发送 Pull Request 事件（feature → main）
- 验证事件处理
- 显示响应状态

---

## 部署脚本

### save-images.sh
将 Docker 镜像导出为 tar 文件，用于离线部署。

```bash
./save-images.sh
```

**功能说明：**
1. 将 Docker 镜像保存到 `docker-images-export/` 目录
2. 创建 tar 文件：ghh-server_latest.tar、quality-server_latest.tar、quality-frontend_latest.tar、mysql_latest.tar、nginx_latest.tar、alpine_3.19.tar

**输出结果：**
```
docker-images-export/
├── alpine_3.19.tar (7.4M)
├── ghh-server_latest.tar (20M)
├── mysql_latest.tar (901M)
├── nginx_latest.tar (157M)
├── quality-frontend_latest.tar (157M)
└── quality-server_latest.tar (21M)
```

**使用场景：**
- 准备镜像传输到离线服务器
- 创建镜像备份
- 离线部署场景

---

### start.sh
在目标服务器上加载 Docker 镜像并启动所有容器。

```bash
# 升级模式（默认）：更新容器，保留数据
./start.sh

# 恢复模式：完全重装，清空数据库
./start.sh -r

# 显示帮助
./start.sh -h
```

**功能说明：**

**预检查：**
- 检查端口可用性（3306、5001、8080、8081）
- 显示容器状态（运行中/已停止/不存在）
- 检测端口冲突

**升级模式（-u 或默认）：**
1. 从 `docker-images-export/` 加载 Docker 镜像
2. 创建 Docker 网络（github-hub-network）
3. 创建数据目录
4. 启动容器（如已运行且镜像相同则跳过）
5. 显示服务状态

**恢复模式（-r）：**
- 停止并删除所有容器
- 备份数据库到 `data/mysql.backup.YYYYMMDD_HHMMSS`
- 重置数据库目录
- 重建 Docker 网络
- 执行全新安装

**启动的服务：**
| 容器 | 端口 | 说明 |
|-----------|------|-------------|
| quality-mysql | 3306 | MySQL 数据库 |
| quality-server | 5001 | 质量 API 服务器 |
| ghh-server | 8080 | GitHub 缓存服务器 |
| quality-frontend | 8081 | Web 界面 |

**访问地址：**
```
前端界面:       http://<服务器IP>:8081
质量 API:       http://<服务器IP>:5001
GHH 服务端:     http://<服务器IP>:8080
MySQL:          localhost:3306
```

**数据库连接信息：**
- 主机: quality-mysql
- 端口: 3306
- 数据库: github_hub
- 用户名: root
- 密码: root123456

---

### stop.sh
停止并删除所有容器。

```bash
./stop.sh
```

**功能说明：**
- 停止容器：quality-frontend、quality-server、ghh-server、quality-mysql
- 删除容器
- 保留数据卷

---

### deploy.sh
本地部署脚本（开发环境使用）。

```bash
./deploy.sh
```

**功能说明：**
- 检查容器状态
- 如未运行则启动容器
- 部署后显示容器状态

**注意：** 生产环境部署请使用 `save-images.sh` + 目标服务器上的 `start.sh`

---

### install_quality.sh

Quality 项目独立部署脚本，用于单独部署 Quality 服务而不影响 GHH 项目。

```bash
# 升级模式（默认）：更新容器，保留数据
./install_quality.sh

# 升级模式
./install_quality.sh -u

# 恢复模式：完全重装，清空数据库
./install_quality.sh -r

# 显示帮助
./install_quality.sh -h
```

**功能说明：**

**升级模式（-u 或默认）：**
1. 直接在 Dockerfile 中编译 quality-server（多阶段构建）
2. 构建 quality-frontend 镜像（如 frontend/dist 不存在会先构建前端）
3. 创建 Docker 网络（quality-network）
4. 创建数据目录
5. 启动容器（如已运行则跳过）
6. 显示服务状态

**恢复模式（-r）：**
- 停止并删除所有 Quality 相关容器
- 备份数据库到 `data/quality-mysql.backup.YYYYMMDD_HHMMSS`
- 重置数据库目录
- 重建 Docker 网络
- 执行全新安装

**启动的服务：**
| 容器 | 端口 | 说明 |
|-----------|------|-------------|
| quality-mysql | 3306 | MySQL 数据库 |
| quality-server | 5001 | 质量 API 服务器 |
| quality-frontend | 8081 | Web 界面 |

**与 GHH 项目的区别：**
- 独立的 Docker 网络：`quality-network`（不影响 GHH 的 `github-hub-network`）
- 独立的数据目录：`data/quality-mysql`、`data/quality-server`
- 可以在同一台服务器上与 GHH 项目共存
- 使用多阶段构建直接编译 Go 代码，无需预编译二进制

**访问地址：**
```
前端界面:       http://<服务器IP>:8081
质量 API:       http://<服务器IP>:5001
MySQL:          localhost:3306
```

**数据库连接信息：**
- 主机: quality-mysql
- 端口: 3306
- 数据库: github_hub
- 用户名: root
- 密码: root123456

---

## 脚本使用工作流

### 开发工作流
```bash
# 1. 编译所有内容
./prepare.sh

# 2. 运行测试
./test.sh -v

# 3. 本地部署
./deploy.sh

# 4. 测试 webhook
./test_webhook.sh

# 5. 负载测试
./loadtest.sh moderate
```

### 生产部署工作流
```bash
# 在开发机器上：

# 1. 编译所有内容
./prepare.sh

# 2. 保存镜像为 tar 文件
./save-images.sh

# 3. 传输项目目录到目标服务器
rsync -avz /root/dev/github-hub-main/ root@<目标服务器>:/root/github-hub/

# 在目标服务器上：

# 4. 启动所有服务
cd /root/github-hub
./start.sh

# 或使用恢复模式进行全新安装
./start.sh -r
```

### 维护工作流
```bash
# 停止服务
./stop.sh

# 更新代码并重新编译
./prepare.sh

# 保存新镜像
./save-images.sh

# 传输到目标服务器并重启
./start.sh
```

---

### 私有仓库工作流

如果你有私有 Docker 仓库的访问权限，可以使用基于仓库的部署工作流，无需传输 tar 文件。

**仓库配置：**
- 仓库地址：`acr.aishu.cn`
- 仓库路径：`ghh`
- 镜像列表：
  - `acr.aishu.cn/ghh/ghh-server:latest`
  - `acr.aishu.cn/ghh/quality-server:latest`
  - `acr.aishu.cn/ghh/quality-frontend:latest`

#### 构建并推送（开发机器）

```bash
# 方式一：一键构建并推送
./build-push.sh

# 方式二：分步执行
./prepare.sh          # 构建镜像
./push-images.sh      # 推送到仓库（会提示输入密码）
```

**push-images.sh 选项：**
```bash
./push-images.sh              # 推送所有镜像
./push-images.sh ghh-server   # 推送指定镜像
```

#### 拉取并部署（目标服务器）

```bash
# 将 pull-start.sh 传输到目标服务器
scp pull-start.sh root@<目标服务器>:/root/github-hub/

# 在目标服务器上，拉取镜像并启动服务
cd /root/github-hub
./pull-start.sh              # 拉取并启动（升级模式）
./pull-start.sh -r           # 拉取并启动（恢复模式）
```

**使用私有仓库的优势：**
- 无需传输大型 tar 文件
- 部署速度更快
- 更好的版本控制
- 更容易回滚到之前版本

**工作流选择建议：**

| 场景 | 推荐工作流 |
|------|-----------|
| 完全离线网络 | Tar 文件方式（save-images.sh + start.sh） |
| 网络稳定 | 私有仓库方式（build-push.sh + pull-start.sh） |
| 多台目标服务器 | 私有仓库方式（推送一次，多次拉取） |

---

# 开发指南

## 项目结构

```
github-hub/
├── cmd/
│   ├── ghh/                   # ghh CLI 客户端入口
│   ├── ghh-server/            # ghh 服务端入口
│   └── quality-server/        # quality 服务端入口
├── internal/
│   ├── client/                # ghh HTTP 客户端
│   ├── server/                # ghh 服务端处理器
│   ├── storage/               # 工作区存储
│   ├── config/                # 配置加载器
│   ├── version/               # 版本字符串
│   └── quality/
│       ├── api/               # API 处理器
│       ├── handlers/          # 事件处理器
│       ├── models/            # 数据模型
│       ├── storage/           # 存储层
│       └── data/              # Mock 测试数据
├── loadtest/                  # 负载测试工具
├── scripts/
│   └── init-mysql.sql         # 数据库初始化
├── frontend/                  # React + Vite 前端
├── bin/                       # 编译输出
├── test.sh                    # 单元测试脚本
├── test_webhook.sh            # Webhook 模拟脚本
├── loadtest.sh                # 负载测试脚本
├── prepare.sh                 # 准备脚本
└── deploy.sh                  # 部署脚本
```

## 技术栈

- **后端**：Go 1.24
- **数据库**：MySQL
- **前端**：React + Vite + Nginx
- **容器**：Docker
- **基础镜像**：Alpine Linux 3.19

---

# 测试

## 单元测试

使用 `test.sh` 脚本运行所有测试：

```bash
# 运行所有测试
./test.sh

# 显示详细输出
./test.sh -v

# 显示覆盖率
./test.sh -c

# 竞态检测 + 覆盖率
./test.sh -r -c

# 测试特定模块
./test.sh ./internal/quality
```

## 测试覆盖率

- `internal/quality/models` - 94.1% 覆盖率
  - 事件创建和解析
  - 质量检查生成
  - 事件过滤逻辑
- `internal/quality/handlers` - 96.7% 覆盖率
  - Push 事件处理
  - Pull Request 事件处理
- `internal/quality/storage` - Mock 存储实现
  - CRUD 操作
  - 错误处理

## API 测试

### 使用 Web UI
1. 访问 `http://localhost:8081`
2. 点击导航栏中的"Mock测试"
3. 选择测试类型并提交

### 使用 curl

```bash
# 发送测试 PR 事件
curl -X POST http://localhost:5001/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: pull_request" \
  -d @test-pr-event.json

# 模拟预定义事件
curl -X POST http://localhost:5001/api/mock/simulate/push \
  -H "Content-Type: application/json"

# 执行自定义测试
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

# 负载测试

项目包含高性能负载测试工具，用于测试 quality-server 的 webhook 处理能力。

## 快速开始

```bash
# 首次运行需要编译
./loadtest.sh

# 基础测试（100 请求，10 并发）
./loadtest.sh basic

# 中等负载（500 请求，20 并发）
./loadtest.sh moderate

# 重负载（1000 请求，50 并发）
./loadtest.sh heavy

# 压力测试（5000 请求，100 并发）
./loadtest.sh stress

# 极限测试（10000 请求，200 并发）
./loadtest.sh extreme

# PR 事件测试
./loadtest.sh pr

# 限速测试（500 请求，100 QPS）
./loadtest.sh rate
```

## 自定义测试

```bash
# 自定义参数
./loadtest.sh custom -n 1000 -c 50 -type push

# 限速测试
./loadtest.sh custom -n 500 -c 20 -qps 100

# 指定服务器
QUALITY_SERVER_URL=http://localhost:5001 ./loadtest.sh stress
```

## 性能基准

| 场景 | 请求数 | 并发 | 吞吐量 | 成功率 |
|------|--------|------|--------|--------|
| 基础测试 | 100 | 10 | ~1,300 req/s | 100% |
| 中等负载 | 500 | 20 | ~2,600 req/s | 100% |
| 重负载 | 1,000 | 50 | ~6,500 req/s | 100% |
| 压力测试 | 5,000 | 100 | ~4,800 req/s | 100% |
| 极限测试 | 10,000 | 200 | ~5,700 req/s | 100% |

## 运维管理

### 查看容器状态

```bash
docker ps --filter "network=github-hub-network"
```

### 查看日志

```bash
docker logs -f ghh-server
docker logs -f quality-server
docker logs -f quality-frontend
docker logs -f quality-mysql
```

### 停止服务

```bash
docker stop ghh-server quality-server quality-frontend quality-mysql
```

### 备份数据

```bash
# 备份 MySQL
docker exec quality-mysql mysqldump -uroot -proot123456 github_hub > backup.sql

# 备份 ghh-server 缓存
tar -czf ghh-cache-backup.tar.gz data/ghh-server/
```

### 恢复数据

```bash
# 恢复 MySQL
docker exec -i quality-mysql mysql -uroot -proot123456 github_hub < backup.sql

# 恢复 ghh-server 缓存
tar -xzf ghh-cache-backup.tar.gz
```

## 故障排查

### 容器无法启动

1. 检查端口是否被占用：`ss -tlnp | grep 8080`
2. 检查容器是否已存在：`docker ps -a | grep -E 'ghh-server|quality-server|quality-mysql'`
3. 查看容器日志：`docker logs <container_name>`
4. 清理并重新部署：`docker rm -f $(docker ps -aq --filter "network=github-hub-network") && ./deploy.sh`

### 数据库连接失败

1. 检查 MySQL 容器：`docker ps | grep quality-mysql`
2. 检查网络连接：`docker exec quality-server ping quality-mysql`
3. 验证数据库凭据（root/root123456）
4. 等待 MySQL 就绪：`docker logs quality-mysql | grep "ready for connections"`

### Webhook 事件未处理

1. 查看服务器日志：`docker logs quality-server`
2. 检查事件是否符合过滤规则（只处理 main 分支）
3. 验证 GitHub Webhook 配置

## 许可证

MIT License

## 联系方式

如有问题或建议，请联系项目维护者。
