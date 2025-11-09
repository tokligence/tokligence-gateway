#!/bin/bash
# Test: Model Aliases Resolution
# Purpose: Test model alias resolution in routing
# Tests: Aliases defined in config/model_aliases.d/

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Model Aliases Resolution Test"
echo "========================================"
echo ""

# Test 1: Alias resolution on /v1/responses
echo "Test 1: claude-3-5-sonnet-20241022 → claude-3-5-haiku-latest"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if [ "$MODEL" = "claude-3-5-haiku-latest" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Alias resolved correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Expected claude-3-5-haiku-latest, got $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Wildcard alias
echo "Test 2: claude-3-5-sonnet* wildcard alias"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-anything",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if [ "$MODEL" = "claude-3-5-haiku-latest" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Wildcard alias works"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wildcard not resolved, got $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Non-aliased model passes through (OpenAI may return versioned name)
echo "Test 3: Non-aliased model (gpt-4o-mini) passes through"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "gpt-4o-mini",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if echo "$MODEL" | grep -q "gpt-4o-mini"; then
    echo -e "${GREEN}✅ PASS${NC}: Non-aliased model preserved (got: $MODEL)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Model changed unexpectedly to $MODEL"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Alias works with /v1/chat/completions
echo "Test 4: Alias on /v1/chat/completions endpoint"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [{"role": "user", "content": "Hi"}]
  }')

MODEL=$(echo "$RESPONSE" | jq -r '.model // empty')
if [ "$MODEL" = "claude-3-5-haiku-latest" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Alias works on chat completions"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Expected claude-3-5-haiku-latest, got $MODEL"
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
    echo -e "${GREEN}✅ All model alias tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
