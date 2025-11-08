#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.gocache}"

cd "$ROOT"
go test ./internal/httpserver/openai/responses
