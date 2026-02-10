#!/bin/bash
# Load testing script for quality-server
# Easy-to-use wrapper for the Go-based load testing tool

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default server URL
SERVER_URL="${QUALITY_SERVER_URL:-http://10.4.174.125:5001}"

# Check if loadtest binary exists
if [ ! -f "bin/loadtest" ]; then
    echo -e "${YELLOW}Building loadtest tool...${NC}"
    mkdir -p bin
    go build -o bin/loadtest ./loadtest
    if [ $? -ne 0 ]; then
        echo -e "${RED}Failed to build loadtest tool${NC}"
        exit 1
    fi
    echo -e "${GREEN}✓ Build complete${NC}"
fi

# Show usage
show_usage() {
    cat << EOF
用法: $0 [测试场景]

GitHub-Hub Quality Server 负载测试脚本

测试场景:
    basic       - 基础测试 (100请求, 10并发)
    moderate    - 中等负载 (500请求, 20并发)
    heavy       - 重负载 (1000请求, 50并发)
    stress      - 压力测试 (5000请求, 100并发)
    extreme     - 极限测试 (10000请求, 200并发)
    pr          - PR事件测试 (1000请求, 50并发)
    rate        - 限速测试 (500请求, 100 QPS)
    custom      - 自定义参数

环境变量:
    QUALITY_SERVER_URL - 指定服务器地址 (默认: http://10.4.174.125:5001)

示例:
    $0                      # 运行基础测试
    $0 moderate             # 运行中等负载测试
    $0 stress               # 运行压力测试
    QUALITY_SERVER_URL=http://localhost:5001 $0 heavy

自定义测试:
    $0 custom -n 1000 -c 50 -type push
    $0 custom -n 500 -c 20 -qps 100 -type pr

参数说明:
    -n <数量>       总请求数
    -c <数量>       并发连接数
    -type <类型>    事件类型 (push 或 pr)
    -qps <数量>     速率限制 (每秒请求数)
    -timeout <秒>   请求超时时间
EOF
}

# Run load test with specified parameters
run_test() {
    local scenario=$1
    shift

    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}  负载测试场景: ${scenario}${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""

    bin/loadtest -url "$SERVER_URL" "$@"
    local exit_code=$?

    echo ""
    echo -e "${BLUE}========================================${NC}"
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}✓ 测试完成${NC}"
    else
        echo -e "${RED}✗ 测试失败${NC}"
    fi
    echo -e "${BLUE}========================================${NC}"

    return $exit_code
}

# Main script logic
case "${1:-basic}" in
    "basic")
        run_test "基础测试" -n 100 -c 10 -type push
        ;;
    "moderate")
        run_test "中等负载" -n 500 -c 20 -type push
        ;;
    "heavy")
        run_test "重负载" -n 1000 -c 50 -type push
        ;;
    "stress")
        run_test "压力测试" -n 5000 -c 100 -type push
        ;;
    "extreme")
        run_test "极限测试" -n 10000 -c 200 -type push
        ;;
    "pr")
        run_test "PR事件测试" -n 1000 -c 50 -type pr
        ;;
    "rate")
        run_test "限速测试" -n 500 -c 20 -qps 100 -type push
        ;;
    "custom")
        shift
        run_test "自定义测试" "$@"
        ;;
    "-h"|"--help"|"help")
        show_usage
        exit 0
        ;;
    *)
        echo -e "${RED}未知测试场景: $1${NC}"
        echo ""
        show_usage
        exit 1
        ;;
esac
