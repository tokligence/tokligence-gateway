#!/bin/bash
# Test: OpenAI Chat Completions → Anthropic Messages Translation
# Purpose: Verify /v1/chat/completions with claude* models is routed to Anthropic
#          when TOKLIGENCE_CHAT_TO_ANTHROPIC is enabled and Anthropic credentials exist.
#
# Requirements:
#   - Gateway running on http://localhost:8081 (or BASE_URL override)
#   - work_mode=auto (or translation) and TOKLIGENCE_CHAT_TO_ANTHROPIC=on for gatewayd
#   - TOKLIGENCE_ANTHROPIC_API_KEY configured in .env

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Chat → Anthropic Messages Translation Test"
echo "========================================"
echo "Endpoint: /v1/chat/completions"
echo "Model: claude-3-5-haiku-20241022"
echo ""

echo "Test 1: Non-streaming Chat→Anthropic (expects Anthropic message JSON)"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Say hello via Anthropic bridge"}],
    "max_tokens": 50
  }' 2>&1)

CONTENT_TYPE=$(echo "$RESPONSE" | jq -r '.content[0].type // empty')
TEXT=$(echo "$RESPONSE" | jq -r '.content[0].text // empty')
OBJECT=$(echo "$RESPONSE" | jq -r '.object // empty')

if [ "$CONTENT_TYPE" = "text" ] && [ -n "$TEXT" ] && [ -z "$OBJECT" ]; then
  echo -e "${GREEN}✅ PASS${NC}: Got Anthropic-style message response: $TEXT"
  TESTS_PASSED=$((TESTS_PASSED + 1))
else
  echo -e "${RED}❌ FAIL${NC}: Expected Anthropic message JSON"
  echo "Response: $RESPONSE"
  TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

echo "Test 2: Streaming Chat→Anthropic (expects OpenAI chat.completion.chunk SSE)"
STREAM=$(timeout 20 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Count to 3 via Anthropic bridge"}],
    "max_tokens": 30,
    "stream": true
  }' | head -40)

if echo "$STREAM" | grep -q '"object":"chat.completion.chunk"'; then
  echo -e "${GREEN}✅ PASS${NC}: Streaming returns OpenAI chat.completion.chunk SSE"
  TESTS_PASSED=$((TESTS_PASSED + 1))
else
  echo -e "${RED}❌ FAIL${NC}: Missing chat.completion.chunk in SSE stream"
  echo "Stream sample:"
  echo "$STREAM"
  TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

echo "========================================"
echo "Test Results"
echo "========================================"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
  echo -e "${GREEN}✅ Chat→Anthropic bridge tests passed!${NC}"
  exit 0
else
  echo -e "${RED}❌ Some Chat→Anthropic bridge tests failed${NC}"
  exit 1
fi

