#!/bin/bash

# Autobuild script for VLX_FrameFlow
# This script standardizes local compilation for arm64 and amd64 using go build -buildmode=pie.
# It also injects versioning based on the current Git commit/tag.

set -e

if [ "$EUID" -eq 0 ] && [ "$CI" != "true" ]; then
    echo "Error: Please do not run this build script as root."
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Error: go is not installed or not in PATH."
    exit 1
fi

TMP_FRONTEND=""
cleanup_build_temp() {
    [ -n "${TMP_FRONTEND:-}" ] && rm -rf "${TMP_FRONTEND}"
    if [ "$CI" != "true" ]; then
        rm -rf frontend_app/node_modules/ frontend_app/dist/ frontend_app/.svelte-kit/ node_modules/ internal/ui/dist/
        find . \( -name "*.test" -o -name "*.out" -o -name "*.swp" -o -name "*~" -o -name ".DS_Store" -o -name ".goutputstream-*" \) -type f -delete 2>/dev/null || true
    fi
}
trap cleanup_build_temp EXIT

APP_NAME="VLX_FrameFlow"
APP_SRV_NAME="VLX_FrameFlow_SRV"
BUILD_DIR="build"

# Get current git commit/tag for versioning
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "dev")
LDFLAGS="-X main.Version=${GIT_TAG} -X main.Commit=${GIT_COMMIT}"

echo "Building version ${GIT_TAG} (${GIT_COMMIT})"

echo "Building frontend assets..."
if command -v npm >/dev/null 2>&1; then
  if [ "$CI" = "true" ]; then
    ( cd frontend_app && npm ci --no-fund --no-audit && npm run build )
    mkdir -p internal/ui/dist
    rm -rf internal/ui/dist/*
    cp -r frontend_app/dist/* internal/ui/dist/
  else
    TMP_FRONTEND="$(mktemp -d /tmp/vlx_frameflow_ui.XXXXXX)"
    cp -r frontend_app/. "${TMP_FRONTEND}/"
    ( cd "${TMP_FRONTEND}" && npm ci --no-fund --no-audit && npm run build )
    mkdir -p internal/ui/dist
    rm -rf internal/ui/dist/*
    cp -r "${TMP_FRONTEND}/dist/"* internal/ui/dist/
    rm -rf "${TMP_FRONTEND}"
    TMP_FRONTEND=""
  fi
else
  echo "Warning: npm not found; embedding placeholder internal/ui/dist."
  mkdir -p internal/ui/dist
  [ -f internal/ui/dist/index.html ] || echo "<!doctype html><html><body>Frontend not built</body></html>" > internal/ui/dist/index.html
fi

mkdir -p "${BUILD_DIR}"

# Build for AMD64
echo "Compiling for Linux AMD64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildmode=pie -ldflags="${LDFLAGS}" -o "${BUILD_DIR}/${APP_NAME}_amd64" ./cmd/client
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildmode=pie -ldflags="${LDFLAGS}" -o "${BUILD_DIR}/${APP_SRV_NAME}_amd64" ./cmd/server

# Build for ARM64 (e.g. Radxa Rock 5T, Orange Pi 5 Plus)
echo "Compiling for Linux ARM64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildmode=pie -ldflags="${LDFLAGS}" -o "${BUILD_DIR}/${APP_NAME}_arm64" ./cmd/client
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildmode=pie -ldflags="${LDFLAGS}" -o "${BUILD_DIR}/${APP_SRV_NAME}_arm64" ./cmd/server

echo "Compiling frontend for Linux AMD64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildmode=pie -ldflags="${LDFLAGS}" -o "${BUILD_DIR}/vlx_frontend_amd64" ./cmd/frontend

echo "Compiling frontend for Linux ARM64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildmode=pie -ldflags="${LDFLAGS}" -o "${BUILD_DIR}/vlx_frontend_arm64" ./cmd/frontend

echo "Cleaning temporary build artifacts (keeping only clean repo + binaries)..."
echo "Build complete! Binaries are located in the '${BUILD_DIR}' directory."
