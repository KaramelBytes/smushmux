#!/usr/bin/env bash
set -euo pipefail

# Always run from repo root
cd "$(dirname "$0")/.."

echo "Building SmushMux binaries..."

GOFLAGS=${GOFLAGS:-}
LDFLAGS=${LDFLAGS:-}
BIN_NAME=${BIN_NAME:-smushmux}

# Default to host-only build unless TARGETS is set
HOST_GOOS=$(go env GOOS)
HOST_GOARCH=$(go env GOARCH)
TARGETS=${TARGETS:-"${HOST_GOOS}/${HOST_GOARCH}"}

# Prefer pure-Go cross-compiles by default
export CGO_ENABLED=${CGO_ENABLED:-0}

mkdir -p dist

build_one() {
  local goos="$1" goarch="$2"
  local ext=""
  if [[ "$goos" == windows ]]; then ext=".exe"; fi
  local out="${BIN_NAME}-${goos}-${goarch}${ext}"
  echo "- ${goos}/${goarch} -> dist/${out}"
  GOOS="$goos" GOARCH="$goarch" go build ${GOFLAGS} -ldflags "${LDFLAGS}" -o "dist/${out}" .
}

# Iterate selected targets
for t in ${TARGETS}; do
  goos=${t%/*}
  goarch=${t#*/}
  build_one "$goos" "$goarch"
done

echo "Done. Artifacts in ./dist"
