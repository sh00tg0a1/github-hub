#!/usr/bin/env bash
set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
BIN_DIR="$PREFIX/bin"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Detect OS/Arch for picking cross-built binaries when available
detect_os() {
  case "$(uname -s)" in
    Linux*) echo "linux" ;;
    Darwin*) echo "darwin" ;;
    *) echo "" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "amd64" ;;
    aarch64|arm64) echo "arm64" ;;
    *) echo "" ;;
  esac
}

OS_NAME="$(detect_os)"
ARCH_NAME="$(detect_arch)"

pick_bin() {
  local name="$1"    # ghh or ghh-server
  local os="$2"
  local arch="$3"
  local root="$4"
  local candidate=""
  if [[ -n "$os" && -n "$arch" ]]; then
    candidate="$root/bin/${os}-${arch}/${name}"
    if [[ -f "$candidate" ]]; then
      echo "$candidate"
      return
    fi
  fi
  # fallback to root-level bin/<name>
  echo "$root/bin/${name}"
}

CLIENT_BIN="$(pick_bin ghh "$OS_NAME" "$ARCH_NAME" "$ROOT_DIR")"
SERVER_BIN="$(pick_bin ghh-server "$OS_NAME" "$ARCH_NAME" "$ROOT_DIR")"

if [[ ! -f "$CLIENT_BIN" || ! -f "$SERVER_BIN" ]]; then
  echo "缺少二进制文件：请先在仓库根目录执行 make build-static 或自行构建 ghh/ghh-server" >&2
  exit 1
fi

echo "Installing to $BIN_DIR..."
sudo install -d "$BIN_DIR"
sudo install -m 755 "$SERVER_BIN" "$BIN_DIR/"
sudo install -m 755 "$CLIENT_BIN" "$BIN_DIR/"

echo "Done. Installed:"
echo "  - $BIN_DIR/ghh-server (from $SERVER_BIN)"
echo "  - $BIN_DIR/ghh (from $CLIENT_BIN)"

