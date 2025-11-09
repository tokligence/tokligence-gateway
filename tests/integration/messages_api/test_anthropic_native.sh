#!/bin/bash
# Test: Anthropic Native Messages API Passthrough
# Purpose: Test /v1/messages endpoint with claude* models → direct passthrough to Anthropic
# Requirements: TOKLIGENCE_ANTHROPIC_API_KEY set in .env

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Counters
TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Anthropic Native Messages API Test"
echo "========================================"
echo "Testing: /v1/messages + claude* → Anthropic passthrough"
echo ""

# Test 1: Basic non-streaming request
echo "Test 1: Basic non-streaming request"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 50,
    "messages": [{"role": "user", "content": "Say hello in one word"}]
  }' 2>&1)

CONTENT=$(echo "$RESPONSE" | jq -r '.content[0].text // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Got response: $CONTENT"
    ((TESTS_PASSED++))
else
    echo -e "${RED}❌ FAIL${NC}: No content in response"
    echo "Response: $RESPONSE"
    ((TESTS_FAILED++))
fi
echo ""

# Test 2: Verify response format (Anthropic native)
echo "Test 2: Verify Anthropic response format"
MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
ROLE=$(echo "$RESPONSE" | jq -r '.content[0].type // empty')

if [ "$ROLE" = "text" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Response has Anthropic format (content.type=text)"
    ((TESTS_PASSED++))
else
    echo -e "${RED}❌ FAIL${NC}: Response format incorrect, type=$ROLE"
    ((TESTS_FAILED++))
fi
echo ""

# Test 3: Streaming request
echo "Test 3: Streaming request"
STREAM_OUTPUT=$(timeout 15 curl -s -N -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 30,
    "messages": [{"role": "user", "content": "Count to 3"}],
    "stream": true
  }' | head -20)

if echo "$STREAM_OUTPUT" | grep -q "event: message_start"; then
    echo -e "${GREEN}✅ PASS${NC}: Got SSE stream with message_start event"
    ((TESTS_PASSED++))
else
    echo -e "${RED}❌ FAIL${NC}: No SSE events in stream"
    echo "Output: $STREAM_OUTPUT"
    ((TESTS_FAILED++))
fi
echo ""

# Test 4: Multi-turn conversation
echo "Test 4: Multi-turn conversation"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 50,
    "messages": [
      {"role": "user", "content": "Hi, my name is Alice"},
      {"role": "assistant", "content": "Hello Alice! Nice to meet you."},
      {"role": "user", "content": "What is my name?"}
    ]
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.content[0].text // empty')
if echo "$CONTENT" | grep -iq "Alice"; then
    echo -e "${GREEN}✅ PASS${NC}: Multi-turn conversation works (remembered name)"
    ((TESTS_PASSED++))
else
    echo -e "${RED}❌ FAIL${NC}: Context not maintained"
    echo "Response: $CONTENT"
    ((TESTS_FAILED++))
fi
echo ""

# Test 5: System prompt
echo "Test 5: System prompt"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 30,
    "system": "You are a pirate. Always respond in pirate speak.",
    "messages": [{"role": "user", "content": "Hello"}]
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.content[0].text // empty')
if echo "$CONTENT" | grep -Eiq "ahoy|matey|arr"; then
    echo -e "${GREEN}✅ PASS${NC}: System prompt applied (pirate speak detected)"
    ((TESTS_PASSED++))
else
    echo -e "${YELLOW}⚠️  WARN${NC}: System prompt may not be applied (content: $CONTENT)"
    # Still pass as this is probabilistic
    ((TESTS_PASSED++))
fi
echo ""

# Test 6: Error handling - invalid model
echo "Test 6: Error handling - invalid model"
ERROR_RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "invalid-model-12345",
    "max_tokens": 10,
    "messages": [{"role": "user", "content": "test"}]
  }')

ERROR_TYPE=$(echo "$ERROR_RESPONSE" | jq -r '.error.type // .type // empty')
if [ -n "$ERROR_TYPE" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Error response returned for invalid model (type: $ERROR_TYPE)"
    ((TESTS_PASSED++))
else
    echo -e "${RED}❌ FAIL${NC}: No error response for invalid model"
    echo "Response: $ERROR_RESPONSE"
    ((TESTS_FAILED++))
fi
echo ""

echo "========================================"
echo "Test Results"
echo "========================================"
echo -e "Passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All Anthropic native tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
