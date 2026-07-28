#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$ROOT_DIR/huawei-ui"
BACKEND_DIR="$ROOT_DIR/huawei-go"

cd "$FRONTEND_DIR"
npm ci
npm run build

rm -rf "$BACKEND_DIR/static"
mkdir -p "$BACKEND_DIR/static"
cp -a "$FRONTEND_DIR/dist/." "$BACKEND_DIR/static/"

cd "$BACKEND_DIR"
go test ./...
go build -trimpath -o server .
exec ./server
