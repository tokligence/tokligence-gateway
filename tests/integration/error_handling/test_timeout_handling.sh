#!/bin/bash
# Test: Timeout Handling
# Purpose: Test gateway handles timeouts properly
# Tests: Request timeout, streaming timeout

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Timeout Handling Test"
echo "========================================"
echo ""

# Test 1: Short timeout for non-streaming request
echo "Test 1: Request completes within reasonable time (10s timeout)"
START=$(date +%s)
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hi in one word"}],
    "max_tokens": 5
  }')
END=$(date +%s)
DURATION=$((END - START))

if [ $DURATION -lt 10 ] && echo "$RESPONSE" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Request completed in ${DURATION}s"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Request timeout or invalid response"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Streaming request completes without hanging
echo "Test 2: Streaming request completes (15s timeout)"
START=$(date +%s)
STREAM=$(timeout 15 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}],
    "max_tokens": 10,
    "stream": true
  }')
END=$(date +%s)
DURATION=$((END - START))

# Check if stream contains data and [DONE] marker
if echo "$STREAM" | grep -q "data:" && echo "$STREAM" | grep -q "data: \[DONE\]"; then
    echo -e "${GREEN}✅ PASS${NC}: Stream completed properly in ${DURATION}s"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Stream incomplete or timed out"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Anthropic endpoint completes without hanging
echo "Test 3: Anthropic /v1/messages completes (10s timeout)"
START=$(date +%s)
RESPONSE=$(timeout 10 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 5,
    "messages": [{"role": "user", "content": "Hi"}]
  }')
END=$(date +%s)
DURATION=$((END - START))

if [ $DURATION -lt 10 ] && echo "$RESPONSE" | jq -e '.content[0].text' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Response completed in ${DURATION}s"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Response timeout or invalid"
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
    echo -e "${GREEN}✅ All timeout handling tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
