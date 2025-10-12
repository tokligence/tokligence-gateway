#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${TOKEN_EXCHANGE_BASE_URL:-http://localhost:8080}
EMAIL=${MFG_EMAIL:-dev@example.com}

curl -sS -X POST "${BASE_URL}/usage/sync" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"${EMAIL}\"}" | jq .
