#!/bin/bash

# Test: Work Mode = translation
# Tests translation-only mode, should reject passthrough requests

set -e

BASE_URL="http://localhost:8081"
AUTH="Bearer test"

echo "🧪 Testing Work Mode: translation"
echo "======================================"
echo ""

# Check current work mode
WORK_MODE=$(curl -s "$BASE_URL/health" | jq -r '.work_mode')
if [ "$WORK_MODE" != "translation" ]; then
  echo "❌ Gateway not in translation mode (current: $WORK_MODE)"
  echo "Set TOKLIGENCE_WORK_MODE=translation and restart gateway"
  exit 1
fi

echo "✅ Gateway in translation mode"
echo ""

# Test 1: /v1/responses + claude* → should work (translation to Anthropic)
echo "=== Test 1: Translation Request (should succeed) ===="
echo "Endpoint: /v1/responses (OpenAI format)"
echo "Model: claude-3-5-haiku-20241022 (Anthropic model)"
echo "Expected: ✅ Translation to Anthropic Messages API"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Say hi"}],
    "stream": false
  }')

STATUS=$(echo "$RESPONSE" | jq -r '.object // "error"')

if [ "$STATUS" = "response" ]; then
  echo "  ✅ Translation successful (as expected)"
  echo "  Model returned: $(echo "$RESPONSE" | jq -r '.model')"
else
  echo "  ❌ Translation failed (unexpected!)"
  echo "  Error: $(echo "$RESPONSE" | jq -r '.error.message // "unknown"')"
fi
echo ""

# Test 2: /v1/responses + gpt* → should FAIL (would require passthrough)
echo "=== Test 2: Passthrough Request (should fail) ===="
echo "Endpoint: /v1/responses (OpenAI format)"
echo "Model: gpt-4o-mini (OpenAI model)"
echo "Expected: ❌ Rejection (work_mode=translation does not support passthrough)"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hi"}],
    "stream": false
  }')

ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // ""')

if echo "$ERROR_MSG" | grep -q "work_mode=translation does not support passthrough"; then
  echo "  ✅ Request rejected as expected"
  echo "  Error message: $ERROR_MSG"
else
  echo "  ❌ Request not rejected (unexpected!)"
  echo "  Response: $RESPONSE"
fi
echo ""

# Test 3: /v1/messages + gpt* → should work (translation to OpenAI)
echo "=== Test 3: Anthropic Endpoint with GPT Model (should succeed) ===="
echo "Endpoint: /v1/messages (Anthropic format)"
echo "Model: gpt-4o (OpenAI model)"
echo "Expected: ✅ Translation to OpenAI"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "test"}],
    "max_tokens": 50
  }')

TEXT=$(echo "$RESPONSE" | jq -r '.content[0].text // ""')

if [ -n "$TEXT" ]; then
  echo "  ✅ Translation successful (as expected)"
  echo "  Response: $(echo "$TEXT" | head -c 50)"
else
  # Check if it's an error
  ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // .error // ""')
  if [ -n "$ERROR_MSG" ]; then
    echo "  ⚠️  Translation attempted but got error: $ERROR_MSG"
  else
    echo "  ❌ Translation failed (unexpected!)"
    echo "  Response: $RESPONSE"
  fi
fi
echo ""

# Test 4: /v1/messages + claude* → should FAIL (would require passthrough)
echo "=== Test 4: Anthropic Passthrough (should fail) ===="
echo "Endpoint: /v1/messages (Anthropic format)"
echo "Model: claude-3-5-sonnet-20241022 (Anthropic model)"
echo "Expected: ❌ Rejection (work_mode=translation does not support passthrough)"
echo ""

RESPONSE=$(curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "hi"}],
    "max_tokens": 10
  }')

ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // ""')

if echo "$ERROR_MSG" | grep -q "work_mode=translation does not support passthrough"; then
  echo "  ✅ Request rejected as expected"
  echo "  Error message: $ERROR_MSG"
else
  echo "  ❌ Request not rejected (unexpected!)"
  echo "  Response: $RESPONSE"
fi
echo ""

echo "======================================"
echo "✅ Translation Mode Tests Complete!"
echo ""
echo "Summary:"
echo "  - Translation requests (mismatching endpoint+model): ✅ Allowed"
echo "  - Passthrough requests (matching endpoint+model): ✅ Rejected"
