#!/bin/bash
# Shared build script for cross-platform binaries
# Used by both release workflow and CI workflow
#
# Usage: ./scripts/build.sh [target]
#   target: all (default) | linux-windows | darwin
#
# macOS (darwin) binaries are built with CGO enabled because gopsutil requires
# cgo to read per-core CPU statistics on macOS (host_processor_info). Without
# cgo the per-core view is silently empty. Because cgo cannot be cross-compiled
# for darwin from Linux, the darwin target must run on a macOS host.
#
# Linux and Windows use pure-Go implementations, so they are cross-compiled with
# CGO disabled (the default for cross-compilation).

set -e

TARGET="${1:-all}"

# Get version from environment or git tag
VERSION=${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}
LDFLAGS="-s -w -X main.version=${VERSION}"

echo "🔨 Building cross-platform binaries (target: ${TARGET}, version: ${VERSION})..."

build_linux_windows() {
  echo "Building Linux AMD64..."
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o sysmon-linux-amd64 .

  echo "Building Linux ARM64..."
  CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o sysmon-linux-arm64 .

  echo "Building Windows AMD64..."
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o sysmon-windows-amd64.exe .

  echo "Building Windows ARM64..."
  CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o sysmon-windows-arm64.exe .
}

build_darwin() {
  # CGO is required for per-core CPU stats on macOS; must run on a macOS host.
  echo "Building macOS Intel (CGO)..."
  CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -o sysmon-darwin-amd64 .

  echo "Building macOS Apple Silicon (CGO)..."
  CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -o sysmon-darwin-arm64 .
}

case "${TARGET}" in
  linux-windows)
    build_linux_windows
    ;;
  darwin)
    build_darwin
    ;;
  all)
    build_linux_windows
    build_darwin
    ;;
  *)
    echo "Unknown target: ${TARGET}" >&2
    echo "Valid targets: all, linux-windows, darwin" >&2
    exit 1
    ;;
esac

echo "✅ Build completed successfully!"
