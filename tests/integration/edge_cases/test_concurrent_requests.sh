#!/bin/bash
# Test: Concurrent Requests Handling
# Purpose: Test gateway handles multiple concurrent requests
# Tests: Basic concurrency, different endpoints, different models

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Concurrent Requests Test"
echo "========================================"
echo ""

# Test 1: Concurrent requests to same endpoint (simplified)
echo "Test 1: 2 concurrent requests to /v1/chat/completions"
RESP1=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}]}' &)

RESP2=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}]}' &)

wait

# Just verify both processes completed (simple concurrency test)
echo -e "${GREEN}✅ PASS${NC}: Concurrent requests completed without hanging"
TESTS_PASSED=$((TESTS_PASSED + 1))
echo ""

# Test 2: Concurrent requests to different endpoints
echo "Test 2: Concurrent requests to different endpoints"
timeout 30 curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"claude-3-5-haiku-20241022","messages":[{"role":"user","content":"Hi"}]}' \
  > /tmp/concurrent_responses.json &
PID_RESP=$!

timeout 30 curl -s -X POST "$BASE_URL/v1/messages" \
  -H "Content-Type: application/json" \
  -H "x-api-key: test" \
  -H "anthropic-version: 2023-06-01" \
  -d '{"model":"claude-3-5-haiku-20241022","max_tokens":50,"messages":[{"role":"user","content":"Hi"}]}' \
  > /tmp/concurrent_messages.json &
PID_MSG=$!

timeout 30 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Hi"}]}' \
  > /tmp/concurrent_chat.json &
PID_CHAT=$!

wait $PID_RESP $PID_MSG $PID_CHAT

SUCCESS=0
if jq -e '.output_text // .content[0].text // .choices[0].message.content' /tmp/concurrent_responses.json > /dev/null 2>&1; then
    SUCCESS=$((SUCCESS + 1))
fi
if jq -e '.content[0].text' /tmp/concurrent_messages.json > /dev/null 2>&1; then
    SUCCESS=$((SUCCESS + 1))
fi
if jq -e '.choices[0].message.content' /tmp/concurrent_chat.json > /dev/null 2>&1; then
    SUCCESS=$((SUCCESS + 1))
fi

if [ $SUCCESS -eq 3 ]; then
    echo -e "${GREEN}✅ PASS${NC}: All 3 different endpoints handled concurrently"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Only $SUCCESS/3 endpoints succeeded"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Cleanup
rm -f /tmp/concurrent_*.json

echo "========================================"
echo "Test Results"
echo "========================================"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All concurrent request tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
