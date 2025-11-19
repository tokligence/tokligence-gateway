#!/usr/bin/env bash
# Test Gemini native API endpoints through gateway

set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8084}"
TEST_MODEL="${TEST_MODEL:-gemini-2.0-flash-exp}"

echo "=== Testing Gemini Native API ==="
echo "Gateway: $GATEWAY_URL"
echo "Model: $TEST_MODEL"
echo

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass_count=0
fail_count=0

function test_passed() {
    echo -e "${GREEN}✓ PASS${NC}: $1"
    ((pass_count++))
}

function test_failed() {
    echo -e "${RED}✗ FAIL${NC}: $1"
    ((fail_count++))
}

function test_skipped() {
    echo -e "${YELLOW}⊘ SKIP${NC}: $1"
}

# Test 1: List models
echo "Test 1: List models"
response=$(curl -s "${GATEWAY_URL}/v1beta/models")
if echo "$response" | jq -e '.models' > /dev/null 2>&1; then
    test_passed "List models returned valid response"
    echo "$response" | jq '.models[] | .name' | head -3
else
    test_failed "List models failed"
    echo "Response: $response"
fi
echo

# Test 2: Get specific model
echo "Test 2: Get model info for $TEST_MODEL"
response=$(curl -s "${GATEWAY_URL}/v1beta/models/${TEST_MODEL}")
if echo "$response" | jq -e '.name' > /dev/null 2>&1; then
    test_passed "Get model info returned valid response"
    echo "$response" | jq -c '{name, displayName}'
else
    test_failed "Get model info failed"
    echo "Response: $response"
fi
echo

# Test 3: Count tokens
echo "Test 3: Count tokens"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/models/${TEST_MODEL}:countTokens" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "Hello, how are you today?"}]
    }]
  }')

if echo "$response" | jq -e '.totalTokens' > /dev/null 2>&1; then
    test_passed "Count tokens returned valid response"
    tokens=$(echo "$response" | jq '.totalTokens')
    echo "Token count: $tokens"
else
    # Check if quota exceeded
    if echo "$response" | jq -e '.error.code == 429' > /dev/null 2>&1; then
        test_skipped "Count tokens (quota exceeded)"
        echo "Quota error: $(echo "$response" | jq -r '.error.message' | head -1)"
    else
        test_failed "Count tokens failed"
        echo "Response: $response"
    fi
fi
echo

# Test 4: Generate content (non-streaming)
echo "Test 4: Generate content (non-streaming)"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/models/${TEST_MODEL}:generateContent" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "Say hello in one word"}]
    }]
  }')

if echo "$response" | jq -e '.candidates[0].content.parts[0].text' > /dev/null 2>&1; then
    test_passed "Generate content (non-streaming) returned valid response"
    content=$(echo "$response" | jq -r '.candidates[0].content.parts[0].text')
    echo "Generated: $content"

    # Check usage metadata
    if echo "$response" | jq -e '.usageMetadata' > /dev/null 2>&1; then
        echo "Usage: $(echo "$response" | jq -c '.usageMetadata')"
    fi
else
    # Check if quota exceeded
    if echo "$response" | jq -e '.error.code == 429' > /dev/null 2>&1; then
        test_skipped "Generate content (quota exceeded)"
        echo "Quota error: $(echo "$response" | jq -r '.error.message' | head -1)"
    else
        test_failed "Generate content (non-streaming) failed"
        echo "Response: $response"
    fi
fi
echo

# Test 5: Stream generate content
echo "Test 5: Stream generate content"
response=$(curl -s -N -X POST "${GATEWAY_URL}/v1beta/models/${TEST_MODEL}:streamGenerateContent?alt=sse" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "Count from 1 to 3"}]
    }]
  }' 2>&1 | head -20)

if echo "$response" | grep -q "data:"; then
    test_passed "Stream generate content returned SSE events"
    echo "First few SSE events:"
    echo "$response" | head -5
else
    # Check if quota exceeded
    if echo "$response" | jq -e '.error.code == 429' > /dev/null 2>&1; then
        test_skipped "Stream generate content (quota exceeded)"
        echo "Quota error: $(echo "$response" | jq -r '.error.message' | head -1)"
    else
        test_failed "Stream generate content failed"
        echo "Response: $response"
    fi
fi
echo

# Summary
echo "=== Test Summary ==="
echo -e "Passed: ${GREEN}${pass_count}${NC}"
echo -e "Failed: ${RED}${fail_count}${NC}"
echo

if [ "$fail_count" -gt 0 ]; then
    exit 1
else
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
