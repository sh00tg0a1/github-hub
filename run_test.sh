#!/bin/bash
cd "$(dirname "$0")"
exec go test ./internal/storage ./internal/server ./internal/client -v -count=1 -timeout 30s 2>&1
