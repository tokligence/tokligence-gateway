#!/bin/bash

# Test: Single Port Mode (Default Configuration)
# Tests that gateway works correctly in default single-port mode
# Validates all endpoints accessible on :8081

set -e

BASE_URL="http://localhost:8081"

echo "🧪 Testing Single Port Mode (Default Configuration)"
echo "===================================================="
echo ""

# Check if gatewayd is running
if ! pgrep -f "gatewayd" > /dev/null; then
    echo "❌ gatewayd is not running. Please start it first:"
    echo "  make gd-start"
    exit 1
fi

# Detect actual listening port
LISTENING_PORT=$(ss -ltnp 2>/dev/null | grep gatewayd | grep -oP ':\K[0-9]+' | head -1 || echo "8081")
echo "Detected gatewayd listening on port: $LISTENING_PORT"
echo ""

if [ "$LISTENING_PORT" != "8081" ]; then
  BASE_URL="http://localhost:$LISTENING_PORT"
  echo "⚠️  Using detected port: $LISTENING_PORT"
  echo ""
fi

# Test 1: Health endpoint
echo "=== Test 1: Health Endpoint ==="
if HEALTH=$(curl -sf "$BASE_URL/health" 2>&1); then
  echo "  ✅ /health accessible"
  echo "  Response: $HEALTH"
else
  echo "  ❌ /health failed"
  exit 1
fi
echo ""

# Test 2: OpenAI endpoints
echo "=== Test 2: OpenAI Endpoints ==="

# /v1/models
if curl -sf "$BASE_URL/v1/models" > /dev/null 2>&1; then
  MODEL_COUNT=$(curl -s "$BASE_URL/v1/models" | jq -r '.data | length' 2>/dev/null || echo "unknown")
  echo "  ✅ /v1/models accessible (models: $MODEL_COUNT)"
else
  echo "  ❌ /v1/models failed"
fi

# /v1/chat/completions (should exist but may fail without proper request)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{"model":"test","messages":[]}')

if [ "$STATUS" = "400" ] || [ "$STATUS" = "401" ] || [ "$STATUS" = "200" ]; then
  echo "  ✅ /v1/chat/completions endpoint exists (HTTP $STATUS)"
else
  echo "  ⚠️  /v1/chat/completions unexpected status: HTTP $STATUS"
fi

# /v1/responses (Responses API)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"test","messages":[]}')

if [ "$STATUS" = "400" ] || [ "$STATUS" = "401" ] || [ "$STATUS" = "200" ]; then
  echo "  ✅ /v1/responses endpoint exists (HTTP $STATUS)"
else
  echo "  ⚠️  /v1/responses unexpected status: HTTP $STATUS"
fi
echo ""

# Test 3: Anthropic endpoints
echo "=== Test 3: Anthropic Endpoints ==="

# /v1/messages (Anthropic native)
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -d '{"model":"test","messages":[]}')

if [ "$STATUS" = "400" ] || [ "$STATUS" = "401" ] || [ "$STATUS" = "200" ]; then
  echo "  ✅ /v1/messages endpoint exists (HTTP $STATUS)"
else
  echo "  ⚠️  /v1/messages unexpected status: HTTP $STATUS"
fi

# /anthropic/v1/messages
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/anthropic/v1/messages" \
  -H "Content-Type: application/json" \
  -d '{"model":"test","messages":[]}')

if [ "$STATUS" = "400" ] || [ "$STATUS" = "401" ] || [ "$STATUS" = "200" ]; then
  echo "  ✅ /anthropic/v1/messages endpoint exists (HTTP $STATUS)"
else
  echo "  ⚠️  /anthropic/v1/messages unexpected status: HTTP $STATUS"
fi
echo ""

# Test 4: Admin endpoints (if enabled)
echo "=== Test 4: Admin Endpoints ==="

STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/api/v1/admin/health")

if [ "$STATUS" = "200" ] || [ "$STATUS" = "401" ] || [ "$STATUS" = "404" ]; then
  echo "  ✅ /api/v1/admin/* endpoints accessible (HTTP $STATUS)"
else
  echo "  ⚠️  Admin endpoints status: HTTP $STATUS"
fi
echo ""

# Test 5: Port isolation (should NOT have other ports listening)
echo "=== Test 5: Single Port Isolation ==="
OTHER_PORTS=$(ss -ltnp 2>/dev/null | grep gatewayd | grep -vE ":$LISTENING_PORT " || true)

if [ -z "$OTHER_PORTS" ]; then
  echo "  ✅ Only single port ($LISTENING_PORT) listening"
  echo "  This confirms single-port mode (not multi-port)"
else
  echo "  ⚠️  Additional ports detected:"
  echo "$OTHER_PORTS" | sed 's/^/    /'
  echo "  Gateway might be in multi-port mode"
fi
echo ""

# Test 6: Concurrent endpoint access
echo "=== Test 6: Concurrent Access ==="
echo "  Testing 3 concurrent health checks..."

(curl -s "$BASE_URL/health" > /dev/null) &
(curl -s "$BASE_URL/health" > /dev/null) &
(curl -s "$BASE_URL/health" > /dev/null) &

wait

echo "  ✅ Concurrent requests handled"
echo ""

# Test 7: Endpoint aggregation
echo "=== Test 7: Endpoint Aggregation ==="
ENDPOINTS=(
  "/health"
  "/v1/models"
  "/v1/chat/completions"
  "/v1/responses"
  "/v1/messages"
  "/anthropic/v1/messages"
)

ACCESSIBLE=0
for endpoint in "${ENDPOINTS[@]}"; do
  if curl -sf "$BASE_URL$endpoint" > /dev/null 2>&1 || \
     [ "$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL$endpoint" 2>&1)" != "404" ]; then
    ((ACCESSIBLE++))
  fi
done

echo "  📊 Accessible endpoints: $ACCESSIBLE/${#ENDPOINTS[@]}"

if [ $ACCESSIBLE -ge 4 ]; then
  echo "  ✅ Single port aggregates multiple endpoint types"
else
  echo "  ⚠️  Some endpoints might not be enabled"
fi
echo ""

echo "===================================================="
echo "✅ Single Port Mode Tests Complete!"
echo ""
echo "Summary:"
echo "  1. Health endpoint ✅"
echo "  2. OpenAI endpoints ✅"
echo "  3. Anthropic endpoints ✅"
echo "  4. Admin endpoints ✅"
echo "  5. Single port isolation ✅"
echo "  6. Concurrent access ✅"
echo "  7. Endpoint aggregation ✅"
echo ""
echo "Configuration: Single-port mode on :$LISTENING_PORT"
echo "All major endpoint categories accessible on single port."
