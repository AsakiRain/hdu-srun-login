#!/usr/bin/env bash
set -euo pipefail

APP_NAME="hdu-srun-login"
BUILD_DIR="build"
RUN_TESTS=1
CLEAN=1

while [[ $# -gt 0 ]]; do
  case "$1" in
    --skip-tests)
      RUN_TESTS=0
      shift
      ;;
    --no-clean)
      CLEAN=0
      shift
      ;;
    -h|--help)
      cat <<EOF
Usage: ./build_cross_platform.sh [--skip-tests] [--no-clean]

Build release binaries for:
  linux/amd64, linux/arm64, linux/mipsle
  darwin/amd64, darwin/arm64
  windows/amd64
EOF
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      exit 1
      ;;
  esac
done

if [[ "$RUN_TESTS" -eq 1 ]]; then
  echo "==> Running tests"
  go test ./...
fi

if [[ "$CLEAN" -eq 1 ]]; then
  echo "==> Cleaning old release binaries"
  rm -rf "$BUILD_DIR"
fi
mkdir -p "$BUILD_DIR"

build_one() {
  local goos="$1"
  local goarch="$2"
  local output="$3"

  echo "==> Building ${goos}/${goarch} -> ${output}"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "$output" .
}

build_one linux amd64 "${BUILD_DIR}/${APP_NAME}-linux-amd64"
build_one linux arm64 "${BUILD_DIR}/${APP_NAME}-linux-arm64"
build_one linux mipsle "${BUILD_DIR}/${APP_NAME}-linux-mipsle"
build_one darwin amd64 "${BUILD_DIR}/${APP_NAME}-darwin-amd64"
build_one darwin arm64 "${BUILD_DIR}/${APP_NAME}-darwin-arm64"
build_one windows amd64 "${BUILD_DIR}/${APP_NAME}-windows-amd64.exe"

echo "==> Done"
ls -lh \
  "${BUILD_DIR}/${APP_NAME}-linux-amd64" \
  "${BUILD_DIR}/${APP_NAME}-linux-arm64" \
  "${BUILD_DIR}/${APP_NAME}-linux-mipsle" \
  "${BUILD_DIR}/${APP_NAME}-darwin-amd64" \
  "${BUILD_DIR}/${APP_NAME}-darwin-arm64" \
  "${BUILD_DIR}/${APP_NAME}-windows-amd64.exe"
