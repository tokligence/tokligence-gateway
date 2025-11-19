#!/usr/bin/env bash
# Test Gemini OpenAI-compatible API endpoints through gateway

set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8084}"
TEST_MODEL="${TEST_MODEL:-gemini-2.0-flash-exp}"

echo "=== Testing Gemini OpenAI-Compatible API ==="
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

# Test 1: Chat completions (non-streaming)
echo "Test 1: Chat completions (non-streaming)"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/openai/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "'"$TEST_MODEL"'",
    "messages": [
      {"role": "user", "content": "Say hello in one word"}
    ]
  }')

if echo "$response" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    test_passed "Chat completions (non-streaming) returned valid OpenAI format"
    content=$(echo "$response" | jq -r '.choices[0].message.content')
    echo "Response: $content"

    # Check usage
    if echo "$response" | jq -e '.usage' > /dev/null 2>&1; then
        echo "Usage: $(echo "$response" | jq -c '.usage')"
    fi
else
    # Check if quota exceeded
    if echo "$response" | jq -e '.error.code == 429' > /dev/null 2>&1; then
        test_skipped "Chat completions (quota exceeded)"
        echo "Quota error: $(echo "$response" | jq -r '.error.message' | head -1)"
    else
        test_failed "Chat completions (non-streaming) failed"
        echo "Response: $response"
    fi
fi
echo

# Test 2: Chat completions (streaming)
echo "Test 2: Chat completions (streaming)"
response=$(curl -s -N -X POST "${GATEWAY_URL}/v1beta/openai/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "'"$TEST_MODEL"'",
    "messages": [
      {"role": "user", "content": "Count from 1 to 3"}
    ],
    "stream": true
  }' 2>&1 | head -20)

if echo "$response" | grep -q "data:"; then
    test_passed "Chat completions (streaming) returned SSE events in OpenAI format"
    echo "First few SSE events:"
    echo "$response" | head -5

    # Check if contains [DONE] marker
    if echo "$response" | grep -q "\[DONE\]"; then
        echo "✓ Contains [DONE] marker"
    fi
else
    # Check if quota exceeded
    if echo "$response" | jq -e '.error.code == 429' > /dev/null 2>&1; then
        test_skipped "Chat completions streaming (quota exceeded)"
        echo "Quota error: $(echo "$response" | jq -r '.error.message' | head -1)"
    else
        test_failed "Chat completions (streaming) failed"
        echo "Response: $response"
    fi
fi
echo

# Test 3: Multi-turn conversation
echo "Test 3: Multi-turn conversation"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/openai/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "'"$TEST_MODEL"'",
    "messages": [
      {"role": "user", "content": "My name is Alice"},
      {"role": "assistant", "content": "Hello Alice! Nice to meet you."},
      {"role": "user", "content": "What is my name?"}
    ]
  }')

if echo "$response" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    test_passed "Multi-turn conversation handled correctly"
    content=$(echo "$response" | jq -r '.choices[0].message.content')
    echo "Response: $content"

    # Check if model remembered the name
    if echo "$content" | grep -iq "alice"; then
        echo "✓ Model correctly recalled context (Alice)"
    else
        echo "⚠ Model may not have recalled context correctly"
    fi
else
    # Check if quota exceeded
    if echo "$response" | jq -e '.error.code == 429' > /dev/null 2>&1; then
        test_skipped "Multi-turn conversation (quota exceeded)"
        echo "Quota error: $(echo "$response" | jq -r '.error.message' | head -1)"
    else
        test_failed "Multi-turn conversation failed"
        echo "Response: $response"
    fi
fi
echo

# Test 4: Validate OpenAI response structure
echo "Test 4: Validate OpenAI response structure"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/openai/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "'"$TEST_MODEL"'",
    "messages": [
      {"role": "user", "content": "Hi"}
    ]
  }')

has_errors=0

# Check required fields
if ! echo "$response" | jq -e '.id' > /dev/null 2>&1; then
    if ! echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        echo "✗ Missing field: id"
        has_errors=1
    fi
fi

if ! echo "$response" | jq -e '.object' > /dev/null 2>&1; then
    if ! echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        echo "✗ Missing field: object"
        has_errors=1
    fi
fi

if ! echo "$response" | jq -e '.created' > /dev/null 2>&1; then
    if ! echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        echo "✗ Missing field: created"
        has_errors=1
    fi
fi

if ! echo "$response" | jq -e '.model' > /dev/null 2>&1; then
    if ! echo "$response" | jq -e '.error' > /dev/null 2>&1; then
        echo "✗ Missing field: model"
        has_errors=1
    fi
fi

if ! echo "$response" | jq -e '.choices' > /dev/null 2>&1; then
    if echo "$response" | jq -e '.error.code == 429' > /dev/null 2>&1; then
        test_skipped "Response structure validation (quota exceeded)"
        echo "Quota error: $(echo "$response" | jq -r '.error.message' | head -1)"
        has_errors=-1
    else
        echo "✗ Missing field: choices"
        has_errors=1
    fi
fi

if [ $has_errors -eq 0 ]; then
    test_passed "Response has valid OpenAI structure"
    echo "Response structure: $(echo "$response" | jq -c 'keys')"
elif [ $has_errors -eq 1 ]; then
    test_failed "Response structure validation failed"
    echo "Response: $response"
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
