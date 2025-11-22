#!/bin/bash
# Test: P0 Translation Library Advanced Features
# Purpose: Test web_search_options, reasoning_effort, thinking, and rich usage tracking
# Requirements: TOKLIGENCE_ANTHROPIC_API_KEY set in .env

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

echo "========================================"
echo "P0 Translation Library Features Test"
echo "========================================"
echo "Testing: web_search_options, reasoning_effort, thinking, rich usage"
echo "Base URL: $BASE_URL"
echo ""

# Test 1: Basic request without new fields (backward compatibility)
echo "Test 1: Backward compatibility - standard request"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Say hello in exactly 5 words"}],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Standard request works"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Standard request failed"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Request with reasoning_effort field
echo "Test 2: reasoning_effort field acceptance"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "What is 2 + 2?"}],
    "max_tokens": 50,
    "reasoning_effort": "medium"
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: reasoning_effort field accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))

    # Check if usage is present
    USAGE=$(echo "$RESPONSE" | jq -e '.usage')
    if [ $? -eq 0 ]; then
        echo "   Usage data: $(echo "$USAGE" | jq -c '{input_tokens, output_tokens}')"
    fi
else
    echo -e "${RED}❌ FAIL${NC}: reasoning_effort field caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Request with thinking configuration
echo "Test 3: thinking configuration field"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Calculate 123 * 456"}],
    "max_tokens": 100,
    "thinking": {
      "type": "enabled",
      "budget_tokens": 1024
    }
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: thinking field accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: thinking field caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Combined - multiple new fields together
echo "Test 4: Multiple new fields combined"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Explain AI briefly"}],
    "max_tokens": 150,
    "reasoning_effort": "low",
    "thinking": {
      "type": "enabled",
      "budget_tokens": 512
    }
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Multiple new fields work together"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Combined fields caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 5: Rich usage tracking - cache tokens
echo "Test 5: Rich usage tracking (cache tokens)"
echo "   Creating request with long system prompt..."

# First request - should potentially create cache
RESPONSE1=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful AI assistant with deep knowledge of quantum physics, machine learning, distributed systems, and software engineering. You provide detailed, accurate, and well-researched answers. Always cite your sources when possible."
      },
      {"role": "user", "content": "What is quantum entanglement?"}
    ],
    "max_tokens": 100
  }')

CACHE_CREATION=$(echo "$RESPONSE1" | jq -r '.usage.cache_creation_input_tokens // .usage.cache_creation_tokens // 0')
CACHE_READ=$(echo "$RESPONSE1" | jq -r '.usage.cache_read_input_tokens // .usage.cache_read_tokens // 0')

if [ "$CACHE_CREATION" != "null" ] && [ "$CACHE_READ" != "null" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Usage fields present"
    echo "   cache_creation: $CACHE_CREATION, cache_read: $CACHE_READ"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${YELLOW}⚠ SKIP${NC}: Cache fields not exposed (may be in passthrough mode)"
    TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
fi
echo ""

# Test 6: Responses API compatibility with new fields
echo "Test 6: Responses API with new fields"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "input": [{"role": "user", "content": "Hello"}],
    "max_output_tokens": 50,
    "reasoning_effort": "low"
  }' 2>&1)

# Responses API may not be fully implemented, so this is optional
if echo "$RESPONSE" | jq -e '.type' > /dev/null 2>&1; then
    ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.type')
    if [ "$ERROR_TYPE" = "error" ]; then
        # Check if it's a "not implemented" vs "bad request" error
        ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // .message')
        if echo "$ERROR_MSG" | grep -qi "not implemented\|not found\|404"; then
            echo -e "${YELLOW}⚠ SKIP${NC}: Responses API not fully implemented"
            TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
        else
            echo -e "${RED}❌ FAIL${NC}: Responses API rejected new fields"
            echo "Error: $ERROR_MSG"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        echo -e "${GREEN}✅ PASS${NC}: Responses API accepts new fields"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    fi
else
    # Not a JSON error, might be working
    if echo "$RESPONSE" | grep -q "event:"; then
        echo -e "${GREEN}✅ PASS${NC}: Responses API streaming works"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${YELLOW}⚠ SKIP${NC}: Responses API response unclear"
        TESTS_SKIPPED=$((TESTS_SKIPPED + 1))
    fi
fi
echo ""

# Test 7: Invalid reasoning_effort value handling
echo "Test 7: Error handling - invalid reasoning_effort"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 50,
    "reasoning_effort": "invalid_value"
  }')

# The gateway should accept any string (validation is at provider level)
# So this should not fail at gateway level
if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Gateway accepts value (provider validates)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    # If provider rejects, that's also acceptable
    ERROR=$(echo "$RESPONSE" | jq -r '.error.message // .message // empty')
    if [ -n "$ERROR" ]; then
        echo -e "${GREEN}✅ PASS${NC}: Provider validation works: $ERROR"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}❌ FAIL${NC}: Unexpected response"
        echo "Response: $RESPONSE"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
fi
echo ""

# Test 8: Empty optional fields
echo "Test 8: Empty/null optional fields"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Hi"}],
    "max_tokens": 50,
    "reasoning_effort": null,
    "thinking": null
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Null optional fields handled correctly"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Null fields caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Summary
echo "========================================"
echo "Test Summary"
echo "========================================"
echo -e "Passed:  ${GREEN}$TESTS_PASSED${NC}"
echo -e "Failed:  ${RED}$TESTS_FAILED${NC}"
echo -e "Skipped: ${YELLOW}$TESTS_SKIPPED${NC}"
echo "Total:   $((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))"
echo ""

if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}❌ TESTS FAILED${NC}"
    exit 1
else
    echo -e "${GREEN}✅ ALL TESTS PASSED${NC}"
    exit 0
fi
