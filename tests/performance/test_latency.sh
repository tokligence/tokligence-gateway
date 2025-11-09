#!/bin/bash
# Test: Response Latency Measurement
# Purpose: Measure and compare latencies across different modes
# Tests: Passthrough latency, translation latency, endpoint comparison

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
echo "Response Latency Test"
echo "========================================"
echo ""

# Function to measure latency
measure_latency() {
    local endpoint=$1
    local payload=$2
    local headers=$3
    local tmpfile="/tmp/latency_test_$$"

    local start=$(date +%s%3N)  # milliseconds
    timeout 15 curl -s -X POST "$BASE_URL$endpoint" $headers -d "$payload" > "$tmpfile"
    local end=$(date +%s%3N)
    local latency=$((end - start))

    echo "$latency"
    cat "$tmpfile"
    rm -f "$tmpfile"
}

# Test 1: OpenAI passthrough latency (baseline)
echo "Test 1: OpenAI passthrough latency (baseline)"
RESULT=$(measure_latency "/v1/chat/completions" \
    '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}' \
    '-H "Content-Type: application/json" -H "Authorization: Bearer test"')

LATENCY=$(echo "$RESULT" | head -1)
RESPONSE=$(echo "$RESULT" | tail -n +2)

if echo "$RESPONSE" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: OpenAI passthrough - ${LATENCY}ms"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Anthropic passthrough latency
echo "Test 2: Anthropic passthrough latency"
RESULT=$(measure_latency "/v1/messages" \
    '{"model":"claude-3-5-haiku-20241022","max_tokens":5,"messages":[{"role":"user","content":"Hi"}]}' \
    '-H "Content-Type: application/json" -H "x-api-key: test" -H "anthropic-version: 2023-06-01"')

LATENCY=$(echo "$RESULT" | head -1)
RESPONSE=$(echo "$RESULT" | tail -n +2)

if echo "$RESPONSE" | jq -e '.content[0].text' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Anthropic passthrough - ${LATENCY}ms"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: OpenAI→Anthropic translation latency (work_mode=auto)
echo "Test 3: OpenAI→Anthropic translation latency"
RESULT=$(measure_latency "/v1/messages" \
    '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}]}' \
    '-H "Content-Type: application/json" -H "Authorization: Bearer test"')

LATENCY=$(echo "$RESULT" | head -1)
RESPONSE=$(echo "$RESULT" | tail -n +2)

if echo "$RESPONSE" | jq -e '.content[0].text // .model' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: OpenAI→Anthropic translation - ${LATENCY}ms"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: Anthropic→OpenAI translation latency
echo "Test 4: Anthropic→OpenAI translation latency"
RESULT=$(measure_latency "/v1/chat/completions" \
    '{"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hi"}],"max_tokens":5}' \
    '-H "Content-Type: application/json" -H "Authorization: Bearer test"')

LATENCY=$(echo "$RESULT" | head -1)
RESPONSE=$(echo "$RESULT" | tail -n +2)

if echo "$RESPONSE" | jq -e '.choices[0].message.content // .object' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Anthropic→OpenAI translation - ${LATENCY}ms"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 5: Responses API latency
echo "Test 5: Responses API latency (OpenAI delegation)"
RESULT=$(measure_latency "/v1/responses" \
    '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}],"max_completion_tokens":5}' \
    '-H "Content-Type: application/json" -H "Authorization: Bearer test"')

LATENCY=$(echo "$RESULT" | head -1)
RESPONSE=$(echo "$RESULT" | tail -n +2)

if echo "$RESPONSE" | jq -e '.output[0].content[0].text // .model' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Responses API - ${LATENCY}ms"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response"
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
    echo -e "${GREEN}✅ All latency tests passed!${NC}"
    echo -e "${YELLOW}Note: Latency measurements are approximate and vary based on network/API conditions${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
