#!/bin/bash
# Test: Work Mode × All Endpoints
# Purpose: Test all endpoints with different work modes
# Tests: Auto mode smart routing across all endpoints

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
echo "Work Mode × All Endpoints Test"
echo "========================================"
echo "Testing: work_mode=auto smart routing"
echo ""

# Test 1: /v1/responses + gpt* → OpenAI (no translation needed)
echo "Test 1: /v1/responses + gpt* → OpenAI passthrough"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if echo "$MODEL" | grep -q "gpt"; then
    echo -e "${GREEN}✅ PASS${NC}: OpenAI model response (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong model: $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: /v1/responses + claude* → Anthropic (translation)
echo "Test 2: /v1/responses + claude* → Anthropic translation"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if echo "$MODEL" | grep -q "claude"; then
    echo -e "${GREEN}✅ PASS${NC}: Anthropic translation worked (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Translation failed, model: $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: /v1/messages + claude* → Anthropic passthrough
echo "Test 3: /v1/messages + claude* → Anthropic passthrough"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "max_tokens": 50,
    "messages": [{"role": "user", "content": "Hi"}]
  }')

TYPE=$(echo "$RESPONSE" | jq -r '.content[0].type // empty')
if [ "$TYPE" = "text" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Anthropic native format returned"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong format"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: /v1/messages + gpt* → OpenAI→Anthropic translation
echo "Test 4: /v1/messages + gpt* → OpenAI→Anthropic translation"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

TYPE=$(echo "$RESPONSE" | jq -r '.content[0].type // empty')
MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if [ "$TYPE" = "text" ] && echo "$MODEL" | grep -q "gpt"; then
    echo -e "${GREEN}✅ PASS${NC}: Translation to Anthropic format (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Translation failed"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 5: /v1/chat/completions + gpt* → OpenAI passthrough
echo "Test 5: /v1/chat/completions + gpt* → OpenAI passthrough"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

OBJECT=$(echo "$RESPONSE" | jq -r '.object // empty')
if [ "$OBJECT" = "chat.completion" ]; then
    echo -e "${GREEN}✅ PASS${NC}: OpenAI chat completion format"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong format: $OBJECT"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 6: /v1/chat/completions + claude* → Anthropic→OpenAI translation
echo "Test 6: /v1/chat/completions + claude* → Anthropic→OpenAI translation"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

OBJECT=$(echo "$RESPONSE" | jq -r '.object // empty')
MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if [ "$OBJECT" = "chat.completion" ] && echo "$MODEL" | grep -q "claude"; then
    echo -e "${GREEN}✅ PASS${NC}: Translation to OpenAI format (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Translation failed"
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
    echo -e "${GREEN}✅ All work mode × endpoints tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
