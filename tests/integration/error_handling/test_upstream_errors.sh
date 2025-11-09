#!/bin/bash
# Test: Upstream API Error Propagation
# Purpose: Test that upstream errors are properly propagated
# Tests: Invalid model, authentication errors

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Upstream Error Propagation Test"
echo "========================================"
echo ""

# Test 1: Invalid model name (Anthropic)
echo "Test 1: Invalid Anthropic model returns error"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-invalid-model-12345",
    "max_tokens": 10,
    "messages": [{"role": "user", "content": "test"}]
  }')

ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error.type // .type // empty')
if [ -n "$ERROR_TYPE" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Error returned (type: $ERROR_TYPE)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No error response"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Invalid model name (OpenAI)
echo "Test 2: Invalid OpenAI model returns error"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-invalid-model-12345",
    "messages": [{"role": "user", "content": "test"}]
  }')

# OpenAI error format can be {"error": "string"} or {"error": {"message": "..."}}
ERROR=$(echo "$RESPONSE" | jq -r 'if .error | type == "string" then .error else .error.message // empty end')
if [ -n "$ERROR" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Error returned"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No error response"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Missing required field (max_tokens for Anthropic)
echo "Test 3: Missing max_tokens returns validation error"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "test"}]
  }')

ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // empty')
if echo "$ERROR_MSG" | grep -qi "max_tokens"; then
    echo -e "${GREEN}✅ PASS${NC}: Validation error for max_tokens"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong error or no error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Empty messages array
echo "Test 4: Empty messages array returns error"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 10,
    "messages": []
  }')

ERROR=$(echo "$RESPONSE" | jq -r '.error.message // .error // empty')
if [ -n "$ERROR" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Error for empty messages"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No error for empty messages"
    echo "Response: $RESPONSE"
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
    echo -e "${GREEN}✅ All upstream error tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
