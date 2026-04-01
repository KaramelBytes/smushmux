#!/usr/bin/env bash
set -euo pipefail

REPO="${REPO:-KaramelBytes/smushmux}"
VERSION="${VERSION:-latest}"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"
BIN_NAME="${BIN_NAME:-smushmux}"

detect_platform() {
  local os arch
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)
  case "$os" in
    linux) os="linux" ;;
    darwin) os="darwin" ;;
    *) echo "Unsupported OS: $os" >&2; exit 1 ;;
  esac
  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    arm64|aarch64) arch="arm64" ;;
    *) echo "Unsupported ARCH: $arch" >&2; exit 1 ;;
  esac
  echo "$os" "$arch"
}

download() {
  local url=$1 out=$2
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$out"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url"
  else
    echo "Need curl or wget to download assets" >&2
    exit 1
  fi
}

resolve_latest_tag() {
  # Prefer following the releases/latest redirect (portable)
  local loc
  loc=$(curl -fsSLI "https://github.com/$REPO/releases/latest" | tr -d '\r' | awk -F'[:/]' '/^location:/{print $NF; exit}') || true
  if [[ -n "$loc" ]]; then
    echo "$loc"
    return 0
  fi
  # Fallback to API with basic sed (no PCRE dependency)
  curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1
}

install_binary() {
  local src=$1 dst=$2
  # Create BIN_DIR if possible
  mkdir -p "$(dirname "$dst")" 2>/dev/null || true
  if mv "$src" "$dst" 2>/dev/null; then
    echo "Installed to $dst"
    return 0
  fi
  if command -v sudo >/dev/null 2>&1; then
    sudo mv "$src" "$dst"
    echo "Installed to $dst"
  else
    echo "Insufficient permissions to write to $(dirname "$dst"). Set BIN_DIR to a writable path or re-run with sudo." >&2
    exit 1
  fi
}

main() {
  read -r OS ARCH < <(detect_platform)
  local tag
  if [ "$VERSION" = "latest" ]; then
    tag=$(resolve_latest_tag)
    if [[ -z "$tag" ]]; then
      echo "Failed to resolve latest release tag" >&2; exit 1
    fi
  else
    tag="$VERSION"
  fi
  echo "Installing ${BIN_NAME} $tag for $OS/$ARCH..."
  local filename
  if [ "$OS" = "windows" ]; then
    echo "Windows not supported by this script" >&2; exit 1
  fi
  if [ "$OS" = "darwin" ] && [ "$ARCH" = "amd64" ]; then filename="${BIN_NAME}-darwin-amd64"; fi
  if [ "$OS" = "darwin" ] && [ "$ARCH" = "arm64" ]; then filename="${BIN_NAME}-darwin-arm64"; fi
  if [ "$OS" = "linux" ] && [ "$ARCH" = "amd64" ]; then filename="${BIN_NAME}-linux-amd64"; fi
  if [ -z "${filename:-}" ]; then echo "No binary for $OS/$ARCH" >&2; exit 1; fi

  tmp=$(mktemp -d)
  trap 'rm -rf "$tmp"' EXIT
  url="https://github.com/$REPO/releases/download/$tag/$filename"
  download "$url" "$tmp/$filename"
  chmod +x "$tmp/$filename"
  install_binary "$tmp/$filename" "$BIN_DIR/${BIN_NAME}"
}

main "$@"
