#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

version="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
mkdir -p bin
go build -ldflags "-X main.version=${version}" -o bin/caged ./cmd/caged

exec ./bin/caged "$@"
