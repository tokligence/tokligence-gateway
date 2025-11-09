#!/bin/bash
# Test: Streaming Response Formats
# Purpose: Validate SSE format across different endpoints
# Tests: Anthropic SSE, OpenAI SSE, format consistency

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Streaming Response Formats Test"
echo "========================================"
echo ""

# Test 1: Anthropic Messages API streaming
echo "Test 1: /v1/messages Anthropic SSE format"
STREAM=$(timeout 20 curl -s -N -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 30,
    "messages": [{"role": "user", "content": "Count to 3"}],
    "stream": true
  }' | head -30)

# Check for Anthropic SSE events
if echo "$STREAM" | grep -q "event: message_start" && \
   echo "$STREAM" | grep -q "event: content_block_delta"; then
    echo -e "${GREEN}✅ PASS${NC}: Anthropic SSE events present"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Missing Anthropic SSE events"
    echo "Stream sample:" && echo "$STREAM" | head -10
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: OpenAI Chat Completions streaming
echo "Test 2: /v1/chat/completions OpenAI SSE format"
STREAM=$(timeout 20 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Say hi"}],
    "stream": true
  }' | head -20)

# Check for OpenAI SSE format (data: prefix)
if echo "$STREAM" | grep -q "data:"; then
    echo -e "${GREEN}✅ PASS${NC}: OpenAI SSE format present"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Missing OpenAI SSE format"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Responses API streaming (OpenAI→Anthropic translation)
echo "Test 3: /v1/responses streaming with translation"
STREAM=$(timeout 20 curl -s -N -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Hi"}],
    "stream": true
  }' | head -30)

# Check for SSE events
if echo "$STREAM" | grep -q "event:"; then
    echo -e "${GREEN}✅ PASS${NC}: Responses API streaming works"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No SSE events in responses stream"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Stream termination marker
echo "Test 4: Stream properly terminates with [DONE]"
STREAM=$(timeout 20 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}],
    "stream": true
  }')

if echo "$STREAM" | grep -q "data: \[DONE\]"; then
    echo -e "${GREEN}✅ PASS${NC}: Stream has [DONE] marker"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Missing [DONE] marker"
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
    echo -e "${GREEN}✅ All streaming format tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
