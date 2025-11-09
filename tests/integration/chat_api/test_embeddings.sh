#!/bin/bash
# Test: Embeddings API
# Purpose: Test /v1/embeddings endpoint
# Requirements: TOKLIGENCE_OPENAI_API_KEY set in .env

set -e

BASE_URL="${BASE_URL:-http://localhost:8081}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Embeddings API Test"
echo "========================================"
echo ""

# Test 1: Single string embedding
echo "Test 1: Single string embedding"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/embeddings" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "text-embedding-3-small",
    "input": "Hello world"
  }')

OBJECT=$(echo "$RESPONSE" | jq -r '.object // empty')
EMBEDDING_LEN=$(echo "$RESPONSE" | jq -r '.data[0].embedding | length')

if [ "$OBJECT" = "list" ] && [ "$EMBEDDING_LEN" -gt 0 ]; then
    echo -e "${GREEN}✅ PASS${NC}: Got embedding (length: $EMBEDDING_LEN)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Invalid response"
    echo "Response: $RESPONSE" | head -20
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Array of strings
echo "Test 2: Array of strings embedding"
RESPONSE=$(timeout 15 curl -s -X POST "$BASE_URL/v1/embeddings" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "text-embedding-3-small",
    "input": ["Hello", "World"]
  }')

DATA_LEN=$(echo "$RESPONSE" | jq -r '.data | length')
if [ "$DATA_LEN" = "2" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Got 2 embeddings"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Expected 2 embeddings, got $DATA_LEN"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Verify embedding object structure
echo "Test 3: Verify response structure"
OBJ_TYPE=$(echo "$RESPONSE" | jq -r '.data[0].object // empty')
if [ "$OBJ_TYPE" = "embedding" ]; then
    echo -e "${GREEN}✅ PASS${NC}: Correct object type"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Wrong object type: $OBJ_TYPE"
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
    echo -e "${GREEN}✅ All embeddings tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
