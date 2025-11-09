#!/bin/bash
# Test: Routing Precedence and Rules
# Purpose: Test model routing precedence (exact vs wildcard)
# Tests: Exact match priority, wildcard matching, route order

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Routing Precedence Test"
echo "========================================"
echo ""

# Test 1: Exact model match (should use alias)
echo "Test 1: Exact model name matches alias"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if echo "$MODEL" | grep -q "claude-3-5-haiku"; then
    echo -e "${GREEN}✅ PASS${NC}: Exact match resolved to alias (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Exact match failed, got: $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Wildcard pattern match
echo "Test 2: Wildcard pattern (claude-3-5-sonnet*) matches"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-latest",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if echo "$MODEL" | grep -q "claude-3-5-haiku"; then
    echo -e "${GREEN}✅ PASS${NC}: Wildcard match resolved (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wildcard match failed, got: $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Non-matching model passes through
echo "Test 3: Non-matching model preserved"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if echo "$MODEL" | grep -q "gpt-4o-mini"; then
    echo -e "${GREEN}✅ PASS${NC}: Non-aliased model preserved (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Model unexpectedly changed, got: $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Route delegation based on model prefix
echo "Test 4: gpt* model routes to OpenAI"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

# Should get OpenAI response format
MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if echo "$MODEL" | grep -q "gpt"; then
    echo -e "${GREEN}✅ PASS${NC}: gpt* routed correctly (model: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Routing failed, got: $MODEL"
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
    echo -e "${GREEN}✅ All routing precedence tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
