#!/usr/bin/env bash
# Corre localmente los mismos pasos que .github/workflows/ci.yml, en el mismo
# orden, para detectar fallos antes de pushear.
set -euo pipefail

cd "$(dirname "$0")/.."

echo "==> go build ./..."
go build ./...

echo "==> go vet ./..."
go vet ./...

echo "==> golangci-lint run"
golangci-lint run

echo "==> go test ./..."
go test ./...

echo "==> OK"
