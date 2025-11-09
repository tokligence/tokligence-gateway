#!/bin/bash
# Test: Malformed and Edge Case Requests
# Purpose: Test error handling for invalid inputs
# Tests: Malformed JSON, empty fields, missing required fields

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Malformed Requests Test"
echo "========================================"
echo ""

# Test 1: Malformed JSON
echo "Test 1: Malformed JSON returns error"
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{invalid json')

HTTP_CODE=$(timeout 10 curl -s -o /dev/null -w "%{http_code}" -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{invalid json')

if [ "$HTTP_CODE" = "400" ] || echo "$RESPONSE" | grep -qi "error\|invalid"; then
    echo -e "${GREEN}✅ PASS${NC}: Malformed JSON rejected (HTTP $HTTP_CODE)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Malformed JSON not handled properly"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Empty request body
echo "Test 2: Empty request body"
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{}')

if echo "$RESPONSE" | grep -qi "error"; then
    echo -e "${GREEN}✅ PASS${NC}: Empty body rejected"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Empty body not handled"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Missing model field on /v1/chat/completions
echo "Test 3: Missing required 'model' field"
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "messages": [{"role": "user", "content": "test"}]
  }')

# Check if error response
if echo "$RESPONSE" | jq -r '.error // empty' | grep -qi "error\|model\|required"; then
    echo -e "${GREEN}✅ PASS${NC}: Missing model field rejected"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Missing model not caught properly"
    echo "Response: $RESPONSE" | head -3
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Empty model string
echo "Test 4: Empty model string"
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "",
    "messages": [{"role": "user", "content": "test"}]
  }')

if echo "$RESPONSE" | jq -r '.error // empty' | grep -qi "error\|model"; then
    echo -e "${GREEN}✅ PASS${NC}: Empty model string rejected"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Empty model string not caught"
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
    echo -e "${GREEN}✅ All malformed request tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
