#!/usr/bin/env bash
set -euo pipefail

BASE_URL=${TOKEN_EXCHANGE_BASE_URL:-http://localhost:8080}
EMAIL=${1:-${MFG_EMAIL:-dev@example.com}}
ROLES=${ROLES:-consumer}
DISPLAY_NAME=${DISPLAY_NAME:-Tokligence Gateway}

payload=$(jq -n --arg email "$EMAIL" --arg roles "$ROLES" --arg name "$DISPLAY_NAME" '
{
  email: $email,
  roles: ($roles | split(",")),
  display_name: $name
}
')

curl -sS -X POST "${BASE_URL}/users" \
  -H 'Content-Type: application/json' \
  -d "$payload"
