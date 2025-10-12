#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="$ROOT/config/setting.ini"
ENVIRONMENT="dev"
if [[ -f "$ENV_FILE" ]]; then
  ENVIRONMENT=$(awk -F= '/environment/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); if ($2 != "") {print $2}}' "$ENV_FILE" | tail -n1)
  [[ -z "$ENVIRONMENT" ]] && ENVIRONMENT="dev"
fi
CFG_FILE="$ROOT/config/$ENVIRONMENT/gateway.ini"
BASE_URL="http://localhost:8080"
if [[ -f "$CFG_FILE" ]]; then
  VALUE=$(awk -F= '/base_url/ {gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2}' "$CFG_FILE" | tail -n1)
  [[ -n "$VALUE" ]] && BASE_URL="$VALUE"
fi
URL="${SMOKE_URL:-${BASE_URL%/}/providers}"
echo "Pinging ${URL}"
curl -fsS "$URL" >/dev/null
echo "OK"
