#!/bin/bash

# P0.5 Quick Fields Integration Test
# Tests: ParallelToolCalls, User, MaxCompletionTokens

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8081}"

echo "========================================"
echo "P0.5 Quick Fields Integration Test"
echo "========================================"
echo "Testing: parallel_tool_calls, user, max_completion_tokens"
echo "Base URL: $BASE_URL"
echo ""

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Test 1: max_completion_tokens field
echo "Test 1: max_completion_tokens field acceptance"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_completion_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: max_completion_tokens field accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: max_completion_tokens field caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: parallel_tool_calls field
echo "Test 2: parallel_tool_calls field acceptance"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_tokens": 50,
    "parallel_tool_calls": true
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: parallel_tool_calls field accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: parallel_tool_calls field caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: user field
echo "Test 3: user field acceptance"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_tokens": 50,
    "user": "test-user-123"
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: user field accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: user field caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: All three fields combined
echo "Test 4: All P0.5 fields combined"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_completion_tokens": 50,
    "parallel_tool_calls": false,
    "user": "test-user-456"
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: All P0.5 fields accepted together"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Combined P0.5 fields caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 5: Backward compatibility (no new fields)
echo "Test 5: Backward compatibility check"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Backward compatibility maintained"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Backward compatibility broken"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Summary
echo "========================================"
echo "Test Summary"
echo "========================================"
echo "Passed:  $TESTS_PASSED"
echo "Failed:  $TESTS_FAILED"
echo "Skipped: $TESTS_SKIPPED"
echo "Total:   $((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ ALL TESTS PASSED${NC}"
    exit 0
else
    echo -e "${RED}❌ SOME TESTS FAILED${NC}"
    exit 1
fi
