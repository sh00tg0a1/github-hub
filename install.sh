#!/usr/bin/env bash
set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
BIN_DIR="$PREFIX/bin"

echo "Building static binaries..."
make build-static

echo "Installing to $BIN_DIR..."
sudo install -d "$BIN_DIR"
sudo install -m 755 bin/ghh-server "$BIN_DIR/"
sudo install -m 755 bin/ghh "$BIN_DIR/"

echo "Done. Installed:"
echo "  - $BIN_DIR/ghh-server"
echo "  - $BIN_DIR/ghh"

