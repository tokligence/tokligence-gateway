#!/bin/bash
# Test: Large Payload Handling
# Purpose: Test gateway handles large requests properly
# Tests: Large message content, multiple messages

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Large Payload Handling Test"
echo "========================================"
echo ""

# Test 1: Large message content (2000 chars)
echo "Test 1: Large message content (2000 characters)"
LARGE_CONTENT=$(python3 -c "print('x' * 2000)")
RESPONSE=$(timeout 30 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d "{
    \"model\": \"gpt-4o-mini\",
    \"messages\": [{\"role\": \"user\", \"content\": \"$LARGE_CONTENT\"}],
    \"max_tokens\": 10
  }")

CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Large content handled successfully"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Large content failed"
    echo "Response: $RESPONSE" | head -5
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Multiple messages in history (10 messages)
echo "Test 2: Multiple messages in conversation history"
RESPONSE=$(timeout 30 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [
      {"role": "user", "content": "Message 1"},
      {"role": "assistant", "content": "Response 1"},
      {"role": "user", "content": "Message 2"},
      {"role": "assistant", "content": "Response 2"},
      {"role": "user", "content": "Message 3"},
      {"role": "assistant", "content": "Response 3"},
      {"role": "user", "content": "Message 4"},
      {"role": "assistant", "content": "Response 4"},
      {"role": "user", "content": "Message 5"},
      {"role": "assistant", "content": "Response 5"}
    ],
    "max_tokens": 10
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Multiple messages handled"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Multiple messages failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Anthropic large message with translation
echo "Test 3: Large message with Anthropic→OpenAI translation"
RESPONSE=$(timeout 30 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [
      {"role": "user", "content": "Please summarize: Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."}
    ],
    "max_tokens": 50
  }')

OBJECT=$(echo "$RESPONSE" | jq -r '.object // empty')
if [ "$OBJECT" = "chat.completion" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Large message translation worked"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Large message translation failed"
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
    echo -e "${GREEN}✅ All large payload tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
