#!/bin/bash
# GitHub Webhook 模拟脚本
# 用于向 quality-server 发送模拟的 GitHub webhook 事件

# 默认服务器地址
SERVER_URL="${QUALITY_SERVER_URL:-http://10.4.174.125:5001}"

echo "=========================================="
echo "  GitHub Webhook 模拟脚本"
echo "=========================================="
echo "目标服务器: $SERVER_URL"
echo ""

# 颜色输出
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 发送 Push 事件（main 分支）
send_push_event() {
    echo -e "${YELLOW}[1/2] 发送 Push 事件 (main 分支)${NC}"

    curl -X POST "$SERVER_URL/webhook" \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: push" \
        -H "X-GitHub-Delivery: 12345678-1234-1234-1234-123456789abc" \
        -d '{
            "ref": "refs/heads/main",
            "repository": {
                "id": 123456789,
                "node_id": "MDEwOlJlcG9zaXRvcnkxMjM0NTY3ODk=",
                "name": "TestRepo",
                "full_name": "testuser/TestRepo",
                "private": false,
                "owner": {
                    "login": "testuser",
                    "id": 1234567,
                    "type": "User"
                },
                "html_url": "https://github.com/testuser/TestRepo",
                "description": "测试仓库",
                "url": "https://api.github.com/repos/testuser/TestRepo",
                "default_branch": "main"
            },
            "sender": {
                "login": "testuser",
                "id": 1234567,
                "type": "User"
            },
            "pusher": {
                "name": "testuser",
                "email": "testuser@example.com"
            },
            "head_commit": {
                "id": "abc123def4567890abcdef1234567890abcdef12",
                "tree_id": "def1234567890abcdef1234567890abcdef1234",
                "distinct": true,
                "message": "Add new feature\n\nThis commit adds a new feature for testing.",
                "timestamp": "2026-02-07T10:00:00Z",
                "url": "https://github.com/testuser/TestRepo/commit/abc123d",
                "author": {
                    "name": "Test User",
                    "email": "testuser@example.com",
                    "username": "testuser"
                },
                "committer": {
                    "name": "Test User",
                    "email": "testuser@example.com",
                    "username": "testuser"
                },
                "added": [
                    "src/newfeature.go",
                    "tests/newfeature_test.go"
                ],
                "removed": [],
                "modified": [
                    "README.md",
                    "go.mod"
                ]
            },
            "commits": [
                {
                    "id": "abc123def4567890abcdef1234567890abcdef12",
                    "tree_id": "def1234567890abcdef1234567890abcdef1234",
                    "distinct": true,
                    "message": "Add new feature\n\nThis commit adds a new feature for testing.",
                    "timestamp": "2026-02-07T10:00:00Z",
                    "url": "https://github.com/testuser/TestRepo/commit/abc123d",
                    "author": {
                        "name": "Test User",
                        "email": "testuser@example.com",
                        "username": "testuser"
                    },
                    "committer": {
                        "name": "Test User",
                        "email": "testuser@example.com",
                        "username": "testuser"
                    },
                    "added": [
                        "src/newfeature.go",
                        "tests/newfeature_test.go"
                    ],
                    "removed": [],
                    "modified": [
                        "README.md"
                    ]
                }
            ]
        }' \
        -w "\nHTTP Status: %{http_code}\n" \
        -s

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Push 事件发送成功${NC}"
    else
        echo -e "${RED}✗ Push 事件发送失败${NC}"
    fi
    echo ""
}

# 发送 Pull Request 事件（从 feature 分支到 main）
send_pr_event() {
    echo -e "${YELLOW}[2/2] 发送 Pull Request 事件 (feature -> main)${NC}"

    curl -X POST "$SERVER_URL/webhook" \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: pull_request" \
        -H "X-GitHub-Delivery: 87654321-4321-4321-4321-cba987654321" \
        -d '{
            "action": "opened",
            "number": 42,
            "pull_request": {
                "id": 987654321,
                "node_id": "MDExOlB1bGxSZXF1ZXN0OTg3NjU0MzIx",
                "html_url": "https://github.com/testuser/TestRepo/pull/42",
                "number": 42,
                "state": "open",
                "title": "feat: Add new feature",
                "body": "This PR adds a new feature that improves performance.\r\n\r\n## Changes\r\n- Added new feature module\r\n- Updated documentation\r\n- Added unit tests",
                "user": {
                    "login": "contributor",
                    "id": 7654321,
                    "type": "User"
                },
                "base": {
                    "label": "testuser:main",
                    "ref": "main",
                    "sha": "1234567890abcdef1234567890abcdef12345678",
                    "repo": {
                        "id": 123456789,
                        "url": "https://api.github.com/repos/testuser/TestRepo",
                        "name": "TestRepo",
                        "full_name": "testuser/TestRepo"
                    }
                },
                "head": {
                    "label": "contributor:feature/new-feature",
                    "ref": "feature/new-feature",
                    "sha": "abcdef1234567890abcdef1234567890abcdef12",
                    "repo": {
                        "id": 123456789,
                        "url": "https://api.github.com/repos/testuser/TestRepo",
                        "name": "TestRepo",
                        "full_name": "testuser/TestRepo"
                    },
                    "user": {
                        "login": "contributor",
                        "id": 7654321
                    }
                },
                "merged": false,
                "mergeable": true,
                "mergeable_state": "clean",
                "merged_by": null,
                "comments": 3,
                "review_comments": 1,
                "additions": 150,
                "deletions": 25,
                "changed_files": 8,
                "commits": 2,
                "created_at": "2026-02-07T09:00:00Z",
                "updated_at": "2026-02-07T10:00:00Z"
            },
            "repository": {
                "id": 123456789,
                "node_id": "MDEwOlJlcG9zaXRvcnkxMjM0NTY3ODk=",
                "name": "TestRepo",
                "full_name": "testuser/TestRepo",
                "private": false,
                "owner": {
                    "login": "testuser",
                    "id": 1234567,
                    "type": "User"
                },
                "html_url": "https://github.com/testuser/TestRepo",
                "description": "测试仓库",
                "url": "https://api.github.com/repos/testuser/TestRepo",
                "default_branch": "main"
            },
            "sender": {
                "login": "contributor",
                "id": 7654321,
                "type": "User"
            }
        }' \
        -w "\nHTTP Status: %{http_code}\n" \
        -s

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Pull Request 事件发送成功${NC}"
    else
        echo -e "${RED}✗ Pull Request 事件发送失败${NC}"
    fi
    echo ""
}

# 发送 PR synchronize 事件
send_pr_sync_event() {
    echo -e "${YELLOW}[额外] 发送 Pull Request synchronize 事件${NC}"

    curl -X POST "$SERVER_URL/webhook" \
        -H "Content-Type: application/json" \
        -H "X-GitHub-Event: pull_request" \
        -H "X-GitHub-Delivery: 11111111-2222-3333-4444-555555555555" \
        -d '{
            "action": "synchronize",
            "number": 43,
            "pull_request": {
                "id": 111111111,
                "number": 43,
                "state": "open",
                "title": "fix: Update dependencies",
                "user": {
                    "login": "developer",
                    "id": 8888888
                },
                "base": {
                    "ref": "main",
                    "sha": "1111222233334444555566667777888899990000"
                },
                "head": {
                    "ref": "fix/update-deps",
                    "sha": "aaaabbbbccccddddeeeeffff0000111122223333",
                    "repo": {
                        "full_name": "testuser/TestRepo"
                    }
                },
                "additions": 50,
                "deletions": 10,
                "changed_files": 3,
                "commits": 1
            },
            "repository": {
                "full_name": "testuser/TestRepo"
            },
            "sender": {
                "login": "developer"
            }
        }' \
        -w "\nHTTP Status: %{http_code}\n" \
        -s

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ PR synchronize 事件发送成功${NC}"
    else
        echo -e "${RED}✗ PR synchronize 事件发送失败${NC}"
    fi
    echo ""
}

# 主函数
main() {
    # 检查参数
    case "${1:-all}" in
        "push")
            send_push_event
            ;;
        "pr")
            send_pr_event
            ;;
        "sync")
            send_pr_sync_event
            ;;
        "all")
            send_push_event
            send_pr_event
            ;;
        *)
            echo "用法: $0 [push|pr|sync|all]"
            echo ""
            echo "选项:"
            echo "  push   - 发送 Push 事件 (main 分支)"
            echo "  pr     - 发送 Pull Request 事件 (opened)"
            echo "  sync   - 发送 Pull Request synchronize 事件"
            echo "  all    - 发送所有事件 (默认)"
            echo ""
            echo "环境变量:"
            echo "  QUALITY_SERVER_URL - 指定服务器地址 (默认: http://10.4.174.125:5001)"
            echo ""
            echo "示例:"
            echo "  $0                # 发送所有事件"
            echo "  $0 push           # 只发送 Push 事件"
            echo "  $0 pr             # 只发送 PR 事件"
            echo "  QUALITY_SERVER_URL=http://localhost:5001 $0"
            exit 1
            ;;
    esac

    echo "=========================================="
    echo -e "${GREEN}测试完成！${NC}"
    echo "=========================================="
    echo ""
    echo "查看事件列表: curl $SERVER_URL/api/events"
    echo "查看系统状态: curl $SERVER_URL/api/status"
}

# 执行主函数
main "$@"
