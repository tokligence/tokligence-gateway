#!/bin/bash
# Test: Anthropic format → OpenAI Chat Completions API Translation
# Purpose: Test /v1/chat/completions + claude* → Anthropic→OpenAI translation
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
echo "Anthropic→OpenAI Translation Test"
echo "========================================"
echo "Testing: /v1/chat/completions + claude* → OpenAI format"
echo ""

# Test 1: Basic request with claude model
echo "Test 1: Claude model → OpenAI format translation"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Say hi"}]
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Got response"
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
    echo -e "${GREEN}✅ PASS${NC}: Correct format (object=chat.completion)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong format, object=$OBJECT"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: System message
echo "Test 3: System message with claude model"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [
      {"role": "system", "content": "Be very brief"},
      {"role": "user", "content": "What is 2+2?"}
    ]
  }')

CONTENT=$(echo "$RESPONSE" | jq -r '.choices[0].message.content // empty')
if [ -n "$CONTENT" ]; then
    echo -e "${GREEN}✅ PASS${NC}: System message handled"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Failed with system message"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Streaming
echo "Test 4: Streaming with claude model"
STREAM=$(timeout 15 curl -s -N -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Count to 3"}],
    "stream": true
  }' | head -20)

if echo "$STREAM" | grep -q "data:"; then
    echo -e "${GREEN}✅ PASS${NC}: Streaming works"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: No SSE stream"
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
    echo -e "${GREEN}✅ All Anthropic→OpenAI translation tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
