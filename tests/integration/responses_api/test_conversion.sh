#!/usr/bin/env bash
set -euo pipefail

# Navigate to project root (3 levels up from tests/integration/responses_api)
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
export GOCACHE="${GOCACHE:-$ROOT/.gocache}"

cd "$ROOT"
go test ./internal/httpserver/openai/responses
