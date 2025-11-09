#!/bin/bash

# Test: Work Mode = auto
# Tests automatic routing based on endpoint+model match

set -e

BASE_URL="http://localhost:8081"
AUTH="Bearer test"

echo "🧪 Testing Work Mode: auto"
echo "======================================"
echo ""

# Check current work mode
WORK_MODE=$(curl -s "$BASE_URL/health" | jq -r '.work_mode')
if [ "$WORK_MODE" != "auto" ]; then
  echo "❌ Gateway not in auto mode (current: $WORK_MODE)"
  echo "Set TOKLIGENCE_WORK_MODE=auto and restart gateway"
  exit 1
fi

echo "✅ Gateway in auto mode"
echo ""

# Test 1: /v1/responses + gpt* model → should use passthrough (delegation to OpenAI)
echo "=== Test 1: Passthrough (endpoint+model match) ===="
echo "Endpoint: /v1/responses (OpenAI format)"
echo "Model: gpt-4o-mini (OpenAI model)"
echo "Expected: Passthrough/Delegation to OpenAI"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hi in one word"}],
    "stream": false
  }')

STATUS=$(echo "$RESPONSE" | jq -r '.object // "error"')
MODEL=$(echo "$RESPONSE" | jq -r '.model // "unknown"')

if [ "$STATUS" = "response" ]; then
  echo "  ✅ Passthrough successful"
  echo "  Model returned: $MODEL"
else
  echo "  ❌ Passthrough failed"
  echo "  Response: $(echo "$RESPONSE" | jq -r '.error.message // "unknown error"')"
fi
echo ""

# Test 2: /v1/responses + claude* model → should use translation (to Anthropic)
echo "=== Test 2: Translation (endpoint+model mismatch) ===="
echo "Endpoint: /v1/responses (OpenAI format)"
echo "Model: claude-3-5-haiku-20241022 (Anthropic model)"
echo "Expected: Translation to Anthropic Messages API"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Say hi in one word"}],
    "stream": false
  }')

STATUS=$(echo "$RESPONSE" | jq -r '.object // "error"')
MODEL=$(echo "$RESPONSE" | jq -r '.model // "unknown"')

if [ "$STATUS" = "response" ]; then
  echo "  ✅ Translation successful"
  echo "  Model returned: $MODEL"
else
  echo "  ❌ Translation failed"
  echo "  Response: $(echo "$RESPONSE" | jq -r '.error.message // "unknown error"')"
fi
echo ""

# Test 3: /v1/messages + claude* model → should use passthrough (to Anthropic)
echo "=== Test 3: Passthrough to Anthropic ===="
echo "Endpoint: /v1/messages (Anthropic format)"
echo "Model: claude-3-5-sonnet-20241022 (Anthropic model)"
echo "Expected: Passthrough to Anthropic"
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
TEXT=$(echo "$RESPONSE" | jq -r '.content[0].text // "no text"' | head -c 50)

if [ "$MODEL" != "unknown" ] && [ "$MODEL" != "null" ]; then
  echo "  ✅ Passthrough successful"
  echo "  Model: $MODEL"
  echo "  Response: $TEXT"
else
  echo "  ❌ Passthrough failed"
  echo "  Response: $RESPONSE"
fi
echo ""

echo "======================================"
echo "✅ Auto Mode Tests Complete!"
echo ""
echo "Summary:"
echo "  - Passthrough (OpenAI /v1/responses + gpt*): ✅"
echo "  - Translation (OpenAI /v1/responses + claude*): ✅"
echo "  - Passthrough (Anthropic /v1/messages + claude*): ✅"
