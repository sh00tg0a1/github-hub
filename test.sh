#!/bin/bash

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 设置 Go 代理
export GOPROXY=https://goproxy.cn,direct

# 项目根目录
PROJECT_ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$PROJECT_ROOT"

# 默认参数
VERBOSE=""
RACE=""
COVERAGE=""
SPECIFIC_PATH=""

# 显示帮助
show_help() {
    cat << EOF
用法: $0 [选项] [路径]

运行 github-hub 项目的所有测试。

选项:
    -v          显示详细输出
    -r          运行竞态检测
    -c          显示覆盖率
    -h          显示此帮助信息

路径:
    指定要测试的包路径（例如：./internal/quality）
    默认运行所有测试
    支持通配符，例如：./internal/quality/...

示例:
    $0                  # 运行所有测试
    $0 -v              # 运行所有测试（详细输出）
    $0 -r -c           # 运行所有测试（竞态检测 + 覆盖率）
    $0 ./internal/quality/...  # 运行 quality 模块所有测试
    $0 ./internal/server  # 运行 server 包测试

环境变量:
    TEST_TIMEOUT    测试超时时间（秒），默认 120
EOF
}

# 解析命令行参数
while getopts "vrcch" opt; do
    case $opt in
        v)
            VERBOSE="-v"
            ;;
        r)
            RACE="-race"
            ;;
        c)
            COVERAGE="-cover"
            ;;
        h)
            show_help
            exit 0
            ;;
        *)
            show_help
            exit 1
            ;;
    esac
done

shift $((OPTIND-1))

# 获取可选的路径参数
if [ -n "$1" ]; then
    SPECIFIC_PATH="$1"
fi

# 测试超时时间（秒）
TEST_TIMEOUT=${TEST_TIMEOUT:-120}

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  GitHub-Hub 测试脚本${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "${YELLOW}项目根目录: ${NC}$PROJECT_ROOT"
echo -e "${YELLOW}测试选项:   ${NC}${VERBOSE}${RACE}${COVERAGE}${SPECIFIC_PATH:-所有模块}"
echo ""

# 记录开始时间
START_TIME=$(date +%s)

# 统计变量
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 函数：运行测试并记录结果
run_tests() {
    local module_path=$1
    local module_name=$(echo "$module_path" | sed 's|^\./||' | sed 's|/$||')

    echo -e "\n${YELLOW}[测试] ${NC}${module_name}"

    # 构建测试命令
    local test_cmd="go test ${VERBOSE} ${RACE} ${COVERAGE} -timeout ${TEST_TIMEOUT}s ${module_path}"

    # 显示执行的命令
    echo -e "${BLUE}执行: ${NC}${test_cmd}"

    # 执行测试并捕获输出
    local test_output
    test_output=$(eval "${test_cmd} 2>&1")
    local exit_code=$?

    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}✓${NC} ${module_name} 测试通过"
        # 如果启用详细模式，显示测试输出
        if [ -n "$VERBOSE" ]; then
            echo "$test_output" | grep -E "^(PASS|FAIL|RUN|---)" | head -20
        fi
        ((PASSED_TESTS++))

        # 如果启用了覆盖率，显示覆盖率信息
        if [ -n "$COVERAGE" ]; then
            coverage=$(echo "$test_output" | grep -oP 'coverage: \K[\d.]+%' | tail -1)
            if [ -n "$coverage" ]; then
                echo -e "  ${BLUE}覆盖率: ${NC}${coverage}"
            fi
        fi
    else
        echo -e "${RED}✗${NC} ${module_name} 测试失败"
        ((FAILED_TESTS++))
        return 1
    fi

    ((TOTAL_TESTS++))
    return 0
}

# 定义测试模块
declare -a MODULES

if [ -n "$SPECIFIC_PATH" ]; then
    # 测试特定路径
    MODULES=("$SPECIFIC_PATH")
else
    # 测试所有模块
    MODULES=(
        "./cmd/ghh"
        "./internal/client"
        "./internal/server"
        "./internal/storage"
        "./internal/version"
        "./internal/quality/models"
        "./internal/quality/handlers"
        "./internal/quality/storage"
        "./internal/quality/api"
    )
fi

# 运行测试
for module in "${MODULES[@]}"; do
    # 跳过包含通配符的路径检查（如 ./internal/quality/...）
    if [[ "$module" == *"/..."* ]]; then
        run_tests "$module"
    elif [ -d "$module" ]; then
        run_tests "$module"
    else
        echo -e "${RED}警告: 路径不存在 - ${module}${NC}"
    fi
done

# 计算耗时
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

# 输出测试总结
echo ""
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  测试总结${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "总模块数: ${TOTAL_TESTS}"
echo -e "${GREEN}通过:      ${PASSED_TESTS}${NC}"
if [ $FAILED_TESTS -gt 0 ]; then
    echo -e "${RED}失败:      ${FAILED_TESTS}${NC}"
fi
echo -e "耗时:      ${DURATION}秒"
echo ""

# 判断退出码
if [ $FAILED_TESTS -gt 0 ]; then
    echo -e "${RED}有测试失败！${NC}"
    exit 1
else
    echo -e "${GREEN}所有测试通过！${NC}"
    exit 0
fi
