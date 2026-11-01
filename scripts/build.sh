#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
python3 scripts/runtime-manifest.py
CGO_ENABLED=0 go test ./...
AMS_INTEGRATION=1 CGO_ENABLED=0 go test ./internal/aligner
go vet ./...
CGO_ENABLED=0 go tool mygo build -platform darwin/universal,windows/amd64 -skip-dmg -o build
python3 scripts/package.py
