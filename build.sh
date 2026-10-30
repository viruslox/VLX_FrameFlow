#!/bin/bash

# Autobuild script for VLX_FrameFlow
# This script standardizes local compilation for arm64 and amd64 using go build -buildmode=pie.
# It also injects versioning based on the current Git commit/tag.

set -e

if [ "$EUID" -eq 0 ]; then
    echo "Error: Please do not run this build script as root."
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo "Error: go is not installed or not in PATH."
    exit 1
fi

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
  ( cd frontend_app && npm ci && npm run build )
  mkdir -p internal/ui/dist
  rm -rf internal/ui/dist/*
  cp -r frontend_app/dist/* internal/ui/dist/
else
  echo "Warning: npm not found; embedding existing internal/ui/dist."
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

echo "Build complete! Binaries are located in the '${BUILD_DIR}' directory."
