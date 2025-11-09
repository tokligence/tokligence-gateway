#!/bin/bash
# Test: OpenAI format → Anthropic Messages API Translation
# Purpose: Test /v1/messages endpoint with OpenAI format → translates to Anthropic
# Requirements: TOKLIGENCE_ANTHROPIC_API_KEY set in .env

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "OpenAI→Anthropic Translation Test"
echo "========================================"
echo "Testing: OpenAI format → Anthropic backend"
echo ""

# Test 1: Basic OpenAI format request with gpt model
echo "Test 1: OpenAI format (gpt model) → Anthropic translation"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hi"}]
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.content[0].text // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: OpenAI format accepted, got response"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No response"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Verify response is Anthropic format
echo "Test 2: Response format is Anthropic native"
TYPE=$(echo "$RESPONSE" | jq -r '.content[0].type // empty')
if [ "$TYPE" = "text" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Response is Anthropic format (content.type=text)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong format, type=$TYPE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: OpenAI format with system message in messages array
echo "Test 3: System message in messages array (OpenAI style)"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "system", "content": "Be brief"},
      {"role": "user", "content": "Hello"}
    ]
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.content[0].text // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: System message handled"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Failed with system in messages"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Streaming with OpenAI format
echo "Test 4: Streaming with OpenAI format (gpt model)"
STREAM=$(timeout 15 curl -s -N -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Count to 3"}],
    "stream": true
  }' | head -15)

if echo "$STREAM" | grep -q "event: message_start"; then
    echo -e "${GREEN}✅ PASS${NC}: Streaming works with OpenAI format"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No SSE events"
    echo "Output: $STREAM"
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
    echo -e "${GREEN}✅ All OpenAI→Anthropic translation tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
