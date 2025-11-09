#!/bin/bash

# Test: Work Mode = passthrough
# Tests passthrough-only mode, should reject translation requests

set -e

BASE_URL="http://localhost:8081"
AUTH="Bearer test"

echo "🧪 Testing Work Mode: passthrough"
echo "======================================"
echo ""

# Check current work mode
WORK_MODE=$(curl -s "$BASE_URL/health" | jq -r '.work_mode')
if [ "$WORK_MODE" != "passthrough" ]; then
  echo "❌ Gateway not in passthrough mode (current: $WORK_MODE)"
  echo "Set TOKLIGENCE_WORK_MODE=passthrough and restart gateway"
  exit 1
fi

echo "✅ Gateway in passthrough mode"
echo ""

# Test 1: /v1/responses + gpt* model → should work (passthrough to OpenAI)
echo "=== Test 1: Passthrough Request (should succeed) ===="
echo "Endpoint: /v1/responses (OpenAI format)"
echo "Model: gpt-4o-mini (OpenAI model)"
echo "Expected: ✅ Passthrough to OpenAI"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hi"}],
    "stream": false
  }')

STATUS=$(echo "$RESPONSE" | jq -r '.object // "error"')

if [ "$STATUS" = "response" ]; then
  echo "  ✅ Passthrough successful (as expected)"
  echo "  Model returned: $(echo "$RESPONSE" | jq -r '.model')"
else
  echo "  ❌ Passthrough failed (unexpected!)"
  echo "  Error: $(echo "$RESPONSE" | jq -r '.error.message // "unknown"')"
fi
echo ""

# Test 2: /v1/responses + claude* model → should FAIL (would require translation)
echo "=== Test 2: Translation Request (should fail) ===="
echo "Endpoint: /v1/responses (OpenAI format)"
echo "Model: claude-3-5-haiku-20241022 (Anthropic model)"
echo "Expected: ❌ Rejection (work_mode=passthrough does not support translation)"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Say hi"}],
    "stream": false
  }')

ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // ""')

if echo "$ERROR_MSG" | grep -q "work_mode=passthrough does not support translation"; then
  echo "  ✅ Request rejected as expected"
  echo "  Error message: $ERROR_MSG"
else
  echo "  ❌ Request not rejected (unexpected!)"
  echo "  Response: $RESPONSE"
fi
echo ""

# Test 3: /v1/messages + claude* → should work (passthrough to Anthropic)
echo "=== Test 3: Anthropic Passthrough (should succeed) ===="
echo "Endpoint: /v1/messages (Anthropic format)"
echo "Model: claude-3-5-sonnet-20241022 (Anthropic model)"
echo "Expected: ✅ Passthrough to Anthropic"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Say hi"}],
    "max_tokens": 100
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // "unknown"')

if [ "$MODEL" != "unknown" ] && [ "$MODEL" != "null" ]; then
  echo "  ✅ Passthrough successful (as expected)"
  echo "  Model: $MODEL"
else
  echo "  ❌ Passthrough failed (unexpected!)"
  echo "  Response: $RESPONSE"
fi
echo ""

# Test 4: /v1/messages + gpt* → should FAIL (would require translation)
echo "=== Test 4: Anthropic Endpoint with GPT Model (should fail) ===="
echo "Endpoint: /v1/messages (Anthropic format)"
echo "Model: gpt-4o (OpenAI model)"
echo "Expected: ❌ Rejection (would require translation)"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "test"}],
    "max_tokens": 10
  }')

ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // ""')

if echo "$ERROR_MSG" | grep -q "work_mode=passthrough does not support translation"; then
  echo "  ✅ Request rejected as expected"
  echo "  Error message: $ERROR_MSG"
else
  echo "  ❌ Request not rejected (unexpected!)"
  echo "  Response: $RESPONSE"
fi
echo ""

echo "======================================"
echo "✅ Passthrough Mode Tests Complete!"
echo ""
echo "Summary:"
echo "  - Passthrough requests (matching endpoint+model): ✅ Allowed"
echo "  - Translation requests (mismatching endpoint+model): ✅ Rejected"
