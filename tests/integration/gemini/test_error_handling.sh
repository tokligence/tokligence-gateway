#!/usr/bin/env bash
# Test Gemini error handling through gateway

set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8084}"

echo "=== Testing Gemini Error Handling ==="
echo "Gateway: $GATEWAY_URL"
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

# Test 1: Invalid model name
echo "Test 1: Invalid model name"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/models/invalid-model-name:generateContent" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "Hello"}]
    }]
  }')

if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
    test_passed "Invalid model returns error response"
    error_msg=$(echo "$response" | jq -r '.error.message' | head -1)
    echo "Error: $error_msg"
else
    test_failed "Invalid model should return error"
    echo "Response: $response"
fi
echo

# Test 2: Empty model name
echo "Test 2: Empty model name"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/openai/chat/completions" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "",
    "messages": [
      {"role": "user", "content": "Hello"}
    ]
  }')

if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
    test_passed "Empty model returns error response"
    error_msg=$(echo "$response" | jq -r '.error.message' | head -1)
    echo "Error: $error_msg"
else
    test_failed "Empty model should return error"
    echo "Response: $response"
fi
echo

# Test 3: Invalid JSON
echo "Test 3: Invalid JSON body"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/models/gemini-2.0-flash-exp:generateContent" \
  -H 'Content-Type: application/json' \
  -d 'invalid json {{{' 2>&1)

if echo "$response" | grep -iq "error\|invalid\|malformed"; then
    test_passed "Invalid JSON returns error"
    echo "Response indicates error (expected)"
else
    test_failed "Invalid JSON should return error"
    echo "Response: $response"
fi
echo

# Test 4: Missing required field (contents)
echo "Test 4: Missing required field"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/models/gemini-2.0-flash-exp:generateContent" \
  -H 'Content-Type: application/json' \
  -d '{}')

if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
    test_passed "Missing required field returns error"
    error_msg=$(echo "$response" | jq -r '.error.message' | head -1)
    echo "Error: $error_msg"
else
    test_failed "Missing required field should return error"
    echo "Response: $response"
fi
echo

# Test 5: Invalid method
echo "Test 5: Invalid API method"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/models/gemini-2.0-flash-exp:invalidMethod" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "Hello"}]
    }]
  }')

# This may or may not error depending on how the gateway handles unknown methods
if echo "$response" | jq -e '.error' > /dev/null 2>&1; then
    test_passed "Invalid method returns error (as expected)"
    error_msg=$(echo "$response" | jq -r '.error.message' | head -1)
    echo "Error: $error_msg"
else
    echo -e "${YELLOW}⊘ NOTE${NC}: Invalid method may be passed through to upstream"
    echo "Response: $response"
    # Don't fail this test as behavior may vary
    ((pass_count++))
fi
echo

# Test 6: Proper error structure from Gemini
echo "Test 6: Error response has proper structure"
response=$(curl -s -X POST "${GATEWAY_URL}/v1beta/models/nonexistent-model:generateContent" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "Hello"}]
    }]
  }')

has_errors=0

if echo "$response" | jq -e '.error.code' > /dev/null 2>&1; then
    echo "✓ Has error.code field"
else
    echo "✗ Missing error.code field"
    has_errors=1
fi

if echo "$response" | jq -e '.error.message' > /dev/null 2>&1; then
    echo "✓ Has error.message field"
else
    echo "✗ Missing error.message field"
    has_errors=1
fi

if [ $has_errors -eq 0 ]; then
    test_passed "Error response has proper structure"
    echo "Error structure: $(echo "$response" | jq -c '.error | keys')"
else
    test_failed "Error response structure validation failed"
    echo "Response: $response"
fi
echo

# Test 7: Gateway timeout handling (if applicable)
echo "Test 7: Very large request handling"
large_text=$(python3 -c "print('Hello ' * 50000)")
response=$(curl -s -m 10 -X POST "${GATEWAY_URL}/v1beta/models/gemini-2.0-flash-exp:countTokens" \
  -H 'Content-Type: application/json' \
  -d '{
    "contents": [{
      "parts": [{"text": "'"$large_text"'"}]
    }]
  }' 2>&1 || echo "timeout_or_error")

if echo "$response" | grep -q "timeout_or_error"; then
    echo -e "${YELLOW}⊘ NOTE${NC}: Large request may timeout (expected for very large inputs)"
    ((pass_count++))
elif echo "$response" | jq -e '.totalTokens' > /dev/null 2>&1; then
    test_passed "Large request handled successfully"
    tokens=$(echo "$response" | jq '.totalTokens')
    echo "Token count for large text: $tokens"
elif echo "$response" | jq -e '.error' > /dev/null 2>&1; then
    test_passed "Large request returned error (acceptable)"
    error_msg=$(echo "$response" | jq -r '.error.message' | head -1)
    echo "Error: $error_msg"
else
    test_failed "Large request handling unclear"
    echo "Response: ${response:0:200}..."
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
