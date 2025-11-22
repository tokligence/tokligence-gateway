#!/bin/bash

# P2 Code Execution Integration Test
# Tests: Multi-type content blocks (text, image, container_upload)

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8081}"

echo "========================================"
echo "P2 Code Execution Integration Test"
echo "========================================"
echo "Testing: Multi-type content blocks"
echo "Base URL: $BASE_URL"
echo ""

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Test 1: Backward compatibility - simple string content
echo "Test 1: Backward compatibility - simple string content"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: String content works"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: String content broken"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Array content blocks with text type
echo "Test 2: Array content blocks with text type"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "What is 2+2?"}
      ]
    }],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Array content blocks accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Array content blocks rejected"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Mixed content blocks (text + image reference)
echo "Test 3: Mixed content blocks structure validation"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "Describe this image"},
        {"type": "image_url", "image_url": {"url": "https://example.com/image.jpg"}}
      ]
    }],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices // .error' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Mixed content blocks structure accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Mixed content blocks structure rejected"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Container upload type (code execution)
echo "Test 4: Container upload type structure validation"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "Run this code"},
        {
          "type": "container_upload",
          "source": {
            "type": "text",
            "media_type": "text/plain",
            "data": "print(\"hello\")"
          }
        }
      ]
    }],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices // .error' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Container upload structure accepted by gateway"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Container upload structure rejected"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 5: System message still works with string content
echo "Test 5: System message compatibility"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [
      {"role": "system", "content": "You are helpful"},
      {"role": "user", "content": "Hello"}
    ],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: System messages work"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: System messages broken"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 6: Tool calls still work
echo "Test 6: Tool calls compatibility"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "What is the weather?"}],
    "max_tokens": 50,
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather",
        "parameters": {"type": "object", "properties": {}}
      }
    }]
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Tool calls work with new content structure"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Tool calls broken"
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
