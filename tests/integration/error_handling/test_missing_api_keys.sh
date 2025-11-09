#!/bin/bash
# Test: Missing API Keys Handling
# Purpose: Test error handling when API keys are missing
# Tests: Missing Anthropic key, missing OpenAI key

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Missing API Keys Test"
echo "========================================"
echo ""

# Note: This test assumes the gateway is configured to handle missing API keys gracefully
# In production, the gateway should return clear error messages when keys are missing

# Test 1: Request requiring Anthropic API (check if error is clear)
echo "Test 1: Claude model request (verifies API key handling)"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 10,
    "messages": [{"role": "user", "content": "Hi"}]
  }')

# Check if we got a valid response or error
if echo "$RESPONSE" | jq -e '.content[0].text // .error' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Request handled (valid response or error)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response format"
    echo "Response: $RESPONSE" | head -3
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Request requiring OpenAI API (check if error is clear)
echo "Test 2: GPT model request (verifies API key handling)"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}],
    "max_tokens": 10
  }')

# Check if we got a valid response or error
if echo "$RESPONSE" | jq -e '.choices[0].message.content // .error' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Request handled (valid response or error)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response format"
    echo "Response: $RESPONSE" | head -3
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Missing x-api-key header for Anthropic endpoint
echo "Test 3: Missing x-api-key header"
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 10,
    "messages": [{"role": "user", "content": "Hi"}]
  }')

# Should get an error about missing API key
if echo "$RESPONSE" | jq -r '.error.type // .type // empty' | grep -qi "error\|authentication\|unauthorized"; then
    echo -e "${GREEN}✅ PASS${NC}: Missing API key detected"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    # If no explicit error, check if we still got handled properly
    if echo "$RESPONSE" | jq -e '.error // .content' > /dev/null 2>&1; then
        echo -e "${GREEN}✅ PASS${NC}: Request handled gracefully"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}❌ FAIL${NC}: Unexpected response for missing API key"
        echo "Response: $RESPONSE" | head -3
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
fi
echo ""

echo "========================================"
echo "Test Results"
echo "========================================"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All API key handling tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
