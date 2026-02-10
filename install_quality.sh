#!/bin/bash
# Quality 项目独立部署脚本
#
# 使用方法:
#   ./install_quality.sh                   # 升级模式（默认）
#   ./install_quality.sh -u                # 升级模式
#   ./install_quality.sh -r                # 恢复模式（完全重装，清理数据库）
#   ./install_quality.sh -h                # 显示帮助信息

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# ============================================================
# 配置
# ============================================================
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

NETWORK_NAME="quality-network"
MYSQL_ROOT_PASSWORD="root123456"
MYSQL_DATABASE="github_hub"
MYSQL_PORT=3306
QUALITY_SERVER_PORT=5001
FRONTEND_PORT=8081

CONTAINERS=(
    "quality-frontend"
    "quality-server"
    "quality-mysql"
)

IMAGES=(
    "quality-server:latest"
    "quality-frontend:latest"
)

# ============================================================
# 参数解析
# ============================================================
MODE="upgrade"
FORCE_CLEANUP=false

while getopts "rufh" opt; do
    case $opt in
        r)
            MODE="recover"
            FORCE_CLEANUP=true
            ;;
        u)
            MODE="upgrade"
            ;;
        h)
            echo "Quality 项目独立部署脚本"
            echo ""
            echo "使用方法:"
            echo "  ./install_quality.sh           # 升级模式（默认）：更新容器，保留数据"
            echo "  ./install_quality.sh -u        # 升级模式：更新容器，保留数据"
            echo "  ./install_quality.sh -r        # 恢复模式：完全卸载后重装，清理数据库"
            echo "  ./install_quality.sh -h        # 显示帮助信息"
            echo ""
            echo "说明:"
            echo "  此脚本独立部署 Quality 项目，不会影响 GHH 项目"
            echo "  Quality 项目包含：quality-server, quality-frontend, quality-mysql"
            exit 0
            ;;
        \?)
            echo -e "${RED}无效选项: -$OPTARG${NC}"
            echo "使用 -h 查看帮助信息"
            exit 1
            ;;
    esac
done

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Quality 项目独立部署${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

if [ "$MODE" = "recover" ]; then
    echo -e "${YELLOW}模式: ${RED}恢复模式 (完全重装，清理数据库)${NC}"
    echo -e "${RED}警告: 此操作将删除所有容器和数据库数据！${NC}"
    echo -ne "确认继续? [y/N] "
    read -r confirm
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        echo "操作已取消"
        exit 0
    fi
else
    echo -e "${YELLOW}模式: ${GREEN}升级模式 (更新容器，保留数据)${NC}"
fi
echo ""

# ============================================================
# 步骤 1: 构建 Docker 镜像
# ============================================================
echo -e "${YELLOW}[步骤 1/4] 构建 Docker 镜像...${NC}"
echo ""

# 检查必要的文件
if [ ! -f "Dockerfile_quality_server" ]; then
    echo -e "  ${RED}✗${NC} Dockerfile_quality_server 不存在"
    exit 1
fi

if [ ! -f "Dockerfile_frontend" ]; then
    echo -e "  ${RED}✗${NC} Dockerfile_frontend 不存在"
    exit 1
fi

# 检查 frontend/dist 是否存在
if [ ! -d "frontend/dist" ]; then
    echo -e "  ${YELLOW}!${NC} frontend/dist 不存在"
    echo -e "  ${YELLOW}正在构建前端...${NC}"
    if [ -d "frontend" ]; then
        cd frontend
        npm config set registry https://registry.npmmirror.com
        echo -e "  安装依赖..."
        if command -v pnpm &> /dev/null; then
            pnpm install
        elif command -v cnpm &> /dev/null; then
            cnpm install
        else
            npm install
        fi
        echo -e "  构建前端..."
        if command -v pnpm &> /dev/null; then
            pnpm build
        elif command -v cnpm &> /dev/null; then
            cnpm run build
        else
            npm run build
        fi
        cd "$PROJECT_ROOT"
    else
        echo -e "  ${RED}✗${NC} frontend 目录不存在"
        exit 1
    fi
fi

# 预先拉取基础镜像，避免构建时卡住
echo -e "  ${YELLOW}预先拉取基础镜像...${NC}"
echo -e "    拉取 golang:1.24-alpine..."
docker pull golang:1.24-alpine >/dev/null 2>&1 || true
echo -e "    拉取 alpine:3.19..."
docker pull alpine:3.19 >/dev/null 2>&1 || true
echo -e "    拉取 nginx:latest..."
docker pull nginx:latest >/dev/null 2>&1 || true
echo -e "    拉取 mysql:latest..."
docker pull mysql:latest >/dev/null 2>&1 || true
echo -e "  ${GREEN}✓${NC} 基础镜像拉取完成"
echo ""

# 构建 quality-server 镜像
echo -e "  构建 quality-server 镜像..."
build_output=$(docker build -f Dockerfile_quality_server -t quality-server:latest . 2>&1)
if [ $? -eq 0 ]; then
    echo "$build_output" | while IFS= read -r line; do
        echo -e "    $line"
    done
    echo -e "  ${GREEN}✓${NC} quality-server 镜像构建完成"
else
    echo "$build_output" | while IFS= read -r line; do
        echo -e "    $line"
    done
    echo -e "  ${RED}✗${NC} quality-server 镜像构建失败"
    exit 1
fi

# 构建 quality-frontend 镜像
echo -e "  构建 quality-frontend 镜像..."
build_output=$(docker build -f Dockerfile_frontend -t quality-frontend:latest . 2>&1)
if [ $? -eq 0 ]; then
    echo "$build_output" | while IFS= read -r line; do
        echo -e "    $line"
    done
    echo -e "  ${GREEN}✓${NC} quality-frontend 镜像构建完成"
else
    echo "$build_output" | while IFS= read -r line; do
        echo -e "    $line"
    done
    echo -e "  ${RED}✗${NC} quality-frontend 镜像构建失败"
    exit 1
fi

echo ""
echo -e "  ${GREEN}✓${NC} 所有镜像构建完成"
echo ""

# ============================================================
# 恢复模式：完全清理
# ============================================================
if [ "$MODE" = "recover" ]; then
    echo -e "${YELLOW}[恢复模式] 清理现有容器和数据...${NC}"
    echo ""

    # 停止并删除所有容器
    for container in "${CONTAINERS[@]}"; do
        if docker ps -a --format '{{.Names}}' | grep -q "^${container}$"; then
            echo -e "  删除容器 ${YELLOW}${container}${NC}..."
            docker stop "$container" >/dev/null 2>&1 || true
            docker rm "$container" >/dev/null 2>&1
            echo -e "    ${GREEN}✓${NC} 已删除"
        fi
    done

    # 清理数据库数据（警告：不可恢复！）
    echo -e "  清理数据库数据..."
    if [ -d "${PROJECT_ROOT}/data/quality-mysql" ]; then
        # 备份数据目录
        BACKUP_DIR="${PROJECT_ROOT}/data/quality-mysql.backup.$(date +%Y%m%d_%H%M%S)"
        mv "${PROJECT_ROOT}/data/quality-mysql" "$BACKUP_DIR"
        echo -e "    ${GREEN}✓${NC} 数据已备份到: $BACKUP_DIR"
    fi

    # 重建空的数据目录
    mkdir -p "${PROJECT_ROOT}/data/quality-mysql"
    echo -e "    ${GREEN}✓${NC} 数据库目录已重置"
    echo ""

    # 删除并重建网络
    if docker network inspect "$NETWORK_NAME" &>/dev/null; then
        docker network rm "$NETWORK_NAME" >/dev/null 2>&1
        echo -e "  ${GREEN}✓${NC} Docker 网络已删除"
    fi
fi

# ============================================================
# 步骤 2: 创建 Docker 网络
# ============================================================
echo -e "${YELLOW}[步骤 2/4] 创建 Docker 网络...${NC}"

if ! docker network inspect "$NETWORK_NAME" &> /dev/null; then
    docker network create "$NETWORK_NAME"
    echo -e "  ${GREEN}✓${NC} 创建网络: $NETWORK_NAME"
else
    echo -e "  ${GREEN}✓${NC} 网络 $NETWORK_NAME 已存在"
fi
echo ""

# ============================================================
# 步骤 3: 创建数据目录
# ============================================================
echo -e "${YELLOW}[步骤 3/4] 创建数据目录...${NC}"

mkdir -p "${PROJECT_ROOT}/data/quality-mysql"
mkdir -p "${PROJECT_ROOT}/data/quality-server"
echo -e "  ${GREEN}✓${NC} 数据目录准备完成"
echo ""

# ============================================================
# 步骤 4: 启动容器
# ============================================================
echo -e "${YELLOW}[步骤 4/4] 启动容器...${NC}"

# 等待函数
wait_for() {
    local name=$1
    local max_wait=${2:-30}
    local count=0

    echo -ne "    等待 ${name} 启动"
    while [ $count -lt $max_wait ]; do
        if docker ps --format '{{.Names}}' | grep -q "^${name}$"; then
            echo -e "\r    ${GREEN}✓${NC} ${name} 已启动"
            return 0
        fi
        sleep 1
        count=$((count + 1))
        echo -ne "."
    done
    echo -e "\r    ${RED}✗${NC} ${name} 启动超时"
    return 1
}

# MySQL 容器
echo -e "  启动 ${YELLOW}quality-mysql${NC}..."
if docker ps --format '{{.Names}}' | grep -q "^quality-mysql$"; then
    if [ "$MODE" = "upgrade" ]; then
        echo -e "    ${YELLOW}!${NC} 升级模式：重新创建容器"
        docker stop quality-mysql &> /dev/null || true
        docker rm quality-mysql &> /dev/null || true
        docker run -d \
            --name quality-mysql \
            --network $NETWORK_NAME \
            -p ${MYSQL_PORT}:3306 \
            -e MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_PASSWORD \
            -e MYSQL_DATABASE=$MYSQL_DATABASE \
            -v "${PROJECT_ROOT}/data/quality-mysql:/var/lib/mysql" \
            -v "${PROJECT_ROOT}/scripts/init-mysql.sql:/docker-entrypoint-initdb.d/init.sql" \
            --restart unless-stopped \
            mysql:latest \
            --character-set-server=utf8mb4 \
            --collation-server=utf8mb4_unicode_ci

        # 等待 MySQL 就绪
        echo -ne "    等待 MySQL 就绪"
        for i in {1..30}; do
            if docker exec quality-mysql mysqladmin ping -h localhost -uroot -p"$MYSQL_ROOT_PASSWORD" &> /dev/null; then
                echo -e "\r    ${GREEN}✓${NC} MySQL 已就绪"
                break
            fi
            sleep 1
            echo -ne "."
        done
    else
        echo -e "    ${YELLOW}!${NC} 已在运行"
    fi
else
    docker rm -f quality-mysql &> /dev/null || true
    docker run -d \
        --name quality-mysql \
        --network $NETWORK_NAME \
        -p ${MYSQL_PORT}:3306 \
        -e MYSQL_ROOT_PASSWORD=$MYSQL_ROOT_PASSWORD \
        -e MYSQL_DATABASE=$MYSQL_DATABASE \
        -v "${PROJECT_ROOT}/data/quality-mysql:/var/lib/mysql" \
        -v "${PROJECT_ROOT}/scripts/init-mysql.sql:/docker-entrypoint-initdb.d/init.sql" \
        --restart unless-stopped \
        mysql:latest \
        --character-set-server=utf8mb4 \
        --collation-server=utf8mb4_unicode_ci

    # 等待 MySQL 就绪
    echo -ne "    等待 MySQL 就绪"
    for i in {1..30}; do
        if docker exec quality-mysql mysqladmin ping -h localhost -uroot -p"$MYSQL_ROOT_PASSWORD" &> /dev/null; then
            echo -e "\r    ${GREEN}✓${NC} MySQL 已就绪"
            break
        fi
        sleep 1
        echo -ne "."
    done
fi

# Quality Server 容器
echo -e "  启动 ${YELLOW}quality-server${NC}..."
if docker ps --format '{{.Names}}' | grep -q "^quality-server$"; then
    if [ "$MODE" = "upgrade" ]; then
        echo -e "    ${YELLOW}!${NC} 升级模式：重新创建容器"
        docker stop quality-server &> /dev/null || true
        docker rm quality-server &> /dev/null || true
        docker run -d \
            --name quality-server \
            --network $NETWORK_NAME \
            -p ${QUALITY_SERVER_PORT}:5001 \
            -v "${PROJECT_ROOT}/data/quality-server:/data" \
            --restart unless-stopped \
            quality-server:latest \
            /app/quality-server \
            -addr :5001 \
            -db "root:${MYSQL_ROOT_PASSWORD}@tcp(quality-mysql:3306)/${MYSQL_DATABASE}?parseTime=true" \
            -log-level info
    else
        echo -e "    ${YELLOW}!${NC} 已在运行"
    fi
else
    docker rm -f quality-server &> /dev/null || true
    docker run -d \
        --name quality-server \
        --network $NETWORK_NAME \
        -p ${QUALITY_SERVER_PORT}:5001 \
        -v "${PROJECT_ROOT}/data/quality-server:/data" \
        --restart unless-stopped \
        quality-server:latest \
        /app/quality-server \
        -addr :5001 \
        -db "root:${MYSQL_ROOT_PASSWORD}@tcp(quality-mysql:3306)/${MYSQL_DATABASE}?parseTime=true" \
        -log-level info
fi

# Frontend 容器
echo -e "  启动 ${YELLOW}quality-frontend${NC}..."
if docker ps --format '{{.Names}}' | grep -q "^quality-frontend$"; then
    if [ "$MODE" = "upgrade" ]; then
        echo -e "    ${YELLOW}!${NC} 升级模式：重新创建容器"
        docker stop quality-frontend &> /dev/null || true
        docker rm quality-frontend &> /dev/null || true
        docker run -d \
            --name quality-frontend \
            --network $NETWORK_NAME \
            -p ${FRONTEND_PORT}:80 \
            --restart unless-stopped \
            quality-frontend:latest
    else
        echo -e "    ${YELLOW}!${NC} 已在运行"
    fi
else
    docker rm -f quality-frontend &> /dev/null || true
    docker run -d \
        --name quality-frontend \
        --network $NETWORK_NAME \
        -p ${FRONTEND_PORT}:80 \
        --restart unless-stopped \
        quality-frontend:latest
fi

echo ""
echo -e "  ${GREEN}✓${NC} 所有容器启动完成"
echo ""

# ============================================================
# 显示状态
# ============================================================
echo -e "${YELLOW}[服务状态]${NC}"
echo ""

docker ps --filter "network=${NETWORK_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}  部署完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}访问地址:${NC}"
echo -e "  Frontend:       ${GREEN}http://114.55.41.118:${FRONTEND_PORT}${NC}"
echo -e "  Quality API:    ${GREEN}http://114.55.41.118:${QUALITY_SERVER_PORT}${NC}"
echo -e "  MySQL:          ${GREEN}localhost:${MYSQL_PORT}${NC}"
echo ""
echo -e "${BLUE}数据库连接信息:${NC}"
echo -e "  Host: quality-mysql"
echo -e "  Port: 3306"
echo -e "  Database: $MYSQL_DATABASE"
echo -e "  Username: root"
echo -e "  Password: $MYSQL_ROOT_PASSWORD"
echo ""
echo -e "${BLUE}常用命令:${NC}"
echo -e "  查看日志:       docker logs -f <container-name>"
echo -e "  停止服务:       docker stop quality-frontend quality-server quality-mysql"
echo -e "  重启服务:       ./install_quality.sh"
echo -e "  完全重装:       ./install_quality.sh -r"
echo ""
