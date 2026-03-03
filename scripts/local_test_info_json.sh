#!/bin/bash
# Local test for info.json feature
# Run from repo root: ./scripts/local_test_info_json.sh
# Optional: ./scripts/local_test_info_json.sh --out test_result.txt  writes output to file

set -e
cd "$(dirname "$0")/.."

OUT_FILE=""
[[ "$1" == "--out" && -n "$2" ]] && { OUT_FILE="$2"; shift 2; }

run() {
  if [[ -n "$OUT_FILE" ]]; then
    "$@" 2>&1 | tee -a "$OUT_FILE"
  else
    "$@"
  fi
}

echo "=== Building ==="
run go build -o bin/ghh ./cmd/ghh
run go build -o bin/ghh-server ./cmd/ghh-server

echo ""
echo "=== Running unit tests ==="
run go test ./internal/storage ./internal/server ./internal/client -v -count=1 -timeout 60s

echo ""
echo "=== Manual integration test ==="
echo "1. Start server: GITHUB_TOKEN=<token> bin/ghh-server --addr :8080 --root /tmp/ghh-test-data"
echo "2. Download: bin/ghh --server http://localhost:8080 download --repo <owner/repo> --branch main --dest /tmp/ghh-download --extract"
echo "3. Check: cat /tmp/ghh-download/info.json | jq ."
echo "   Expected fields: repo, branch, commit_sha, commit_message, changed_files"
