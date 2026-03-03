#!/bin/bash
# Run tests and write output to workspace file (for environments where stdout is not captured)
# Usage: ./scripts/run_tests.sh

cd "$(dirname "$0")/.."
OUT=test_output.txt

echo "Running tests, output to $OUT ..."
go test ./... -count=1 -timeout 60s -v 2>&1 > "$OUT"
EXIT=$?
echo "Exit code: $EXIT" >> "$OUT"
echo "Done. See $OUT"
exit $EXIT
