#!/bin/bash
# Test: Anthropic Native Messages API Passthrough (Simplified)
# Purpose: Test /v1/messages endpoint with claude* models
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
echo "Anthropic Native Messages API Test"
echo "========================================"

# Test 1: Basic request
echo "Test 1: Basic non-streaming request"
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-3-5-haiku-20241022","max_tokens":50,"messages":[{"role":"user","content":"Hi"}]}')

TEXT=$(echo "$RESPONSE" | jq -r '.content[0].text // empty')
if [ -n "$TEXT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Got response"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No response"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 2: Response format
echo "Test 2: Verify Anthropic format"
TYPE=$(echo "$RESPONSE" | jq -r '.content[0].type // empty')
if [ "$TYPE" = "text" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Correct format"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong format"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

# Test 3: Streaming
echo "Test 3: Streaming request"
STREAM=$(timeout 10 curl -s -N -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-3-5-haiku-20241022","max_tokens":20,"messages":[{"role":"user","content":"Hi"}],"stream":true}' \
  | head -10)

if echo "$STREAM" | grep -q "event: message_start"; then
    echo -e "${GREEN}✅ PASS${NC}: Streaming works"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No stream events"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi

echo ""
echo "Results: Passed=$TESTS_PASSED Failed=$TESTS_FAILED"

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
