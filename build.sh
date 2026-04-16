#!/usr/bin/env bash
set -euo pipefail

echo "==> Building yggmap"

# --- Frontend ---
echo "==> Building frontend..."
cd web
npm install --silent
npm run build
cd ..

# Copy dist into Go embed directory
echo "==> Copying frontend dist to internal/web/dist..."
rm -rf internal/web/dist
cp -r web/dist internal/web/dist

# --- Backend (cross-platform) ---
echo "==> Building binaries..."
mkdir -p dist

LDFLAGS="-s -w -X github.com/JB-SelfCompany/yggmap/internal/version.Version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')"

# Linux amd64
GOOS=linux   GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/yggmap-linux-amd64    ./cmd/yggmap
# Linux arm64
GOOS=linux   GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/yggmap-linux-arm64    ./cmd/yggmap
# macOS amd64
GOOS=darwin  GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/yggmap-darwin-amd64   ./cmd/yggmap
# macOS arm64 (Apple Silicon)
GOOS=darwin  GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/yggmap-darwin-arm64   ./cmd/yggmap
# Windows amd64
GOOS=windows GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/yggmap-windows-amd64.exe ./cmd/yggmap

echo "==> Build complete! Binaries in dist/"
ls -lh dist/
