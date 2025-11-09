#!/bin/bash
# Test: OpenAI Native Chat Completions API Passthrough
# Purpose: Test /v1/chat/completions + gpt* → OpenAI passthrough
# Requirements: TOKLIGENCE_OPENAI_API_KEY set in .env

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "OpenAI Native Chat Completions Test"
echo "========================================"
echo "Testing: /v1/chat/completions + gpt* → OpenAI"
echo ""

# Test 1: Basic non-streaming request
echo "Test 1: Basic non-streaming request"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hello in one word"}],
    "max_tokens": 10
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Got response: $CONTENT"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No content"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Verify OpenAI response format
echo "Test 2: Verify OpenAI response format"
OBJECT=$(echo "$RESPONSE" | jq -r '.object // empty')
if [ "$OBJECT" = "chat.completion" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Correct OpenAI format (object=chat.completion)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong format, object=$OBJECT"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Streaming request
echo "Test 3: Streaming request"
STREAM=$(timeout 15 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Count to 3"}],
    "max_tokens": 30,
    "stream": true
  }' | head -20)

if echo "$STREAM" | grep -q "data:"; then
    echo -e "${GREEN}✅ PASS${NC}: Streaming works (SSE data: chunks)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No SSE stream"
    echo "Output: $STREAM"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: System message
echo "Test 4: System message"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "system", "content": "Be very brief"},
      {"role": "user", "content": "What is 2+2?"}
    ],
    "max_tokens": 10
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: System message handled: $CONTENT"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Failed with system message"
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
    echo -e "${GREEN}✅ All OpenAI Chat Completions tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
