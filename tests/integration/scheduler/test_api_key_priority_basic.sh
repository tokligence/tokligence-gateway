#!/bin/bash
# Test: API Key Priority Mapping - Basic CRUD Operations
# Description: Tests basic CRUD operations for API key priority mappings with UUID support

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PORT_OFFSET=${PORT_OFFSET:-0}
BASE_PORT=$((8081 + PORT_OFFSET))
ADMIN_PORT=$((8079 + PORT_OFFSET))
BASE_URL="${BASE_URL:-http://localhost:${BASE_PORT}}"
API_KEY="${TOKLIGENCE_API_KEY:-test}"

echo "=== Test: API Key Priority Mapping - Basic CRUD ==="
echo "BASE_URL: $BASE_URL"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

test_passed=0
test_failed=0

# Helper function to check response
check_response() {
    local response="$1"
    local expected_status="$2"
    local test_name="$3"

    actual_status=$(echo "$response" | head -n 1 | cut -d' ' -f2)

    if [ "$actual_status" = "$expected_status" ]; then
        echo -e "${GREEN}✓${NC} $test_name (status: $actual_status)"
        ((test_passed++))
        return 0
    else
        echo -e "${RED}✗${NC} $test_name (expected: $expected_status, got: $actual_status)"
        echo "Response: $response"
        ((test_failed++))
        return 1
    fi
}

echo ""
echo "1. Test LIST empty mappings (should return empty array)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X GET \
    "$BASE_URL/admin/api-key-priority/mappings" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json")
check_response "$response" "200" "LIST empty mappings"

echo ""
echo "2. Test CREATE mapping (should return UUID)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST \
    "$BASE_URL/admin/api-key-priority/mappings" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
        "pattern": "tok_test_*",
        "priority": 2,
        "match_type": "prefix",
        "tenant_id": "test-tenant",
        "tenant_name": "Test Tenant",
        "tenant_type": "internal",
        "description": "Test mapping",
        "created_by": "test-script"
    }')

status=$(echo "$response" | grep "HTTP_STATUS" | cut -d: -f2)
if [ "$status" = "201" ]; then
    # Extract UUID from response
    uuid=$(echo "$response" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo -e "${GREEN}✓${NC} CREATE mapping (UUID: $uuid)"
    ((test_passed++))
else
    echo -e "${RED}✗${NC} CREATE mapping (expected: 201, got: $status)"
    ((test_failed++))
fi

echo ""
echo "3. Test LIST with one mapping (should return array with UUID)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X GET \
    "$BASE_URL/admin/api-key-priority/mappings" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json")
check_response "$response" "200" "LIST with one mapping"

echo ""
echo "4. Test UPDATE mapping (change priority from P2 to P5)"
if [ -n "$uuid" ]; then
    response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X PUT \
        "$BASE_URL/admin/api-key-priority/mappings/$uuid" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -d '{
            "priority": 5,
            "description": "Updated test mapping",
            "enabled": true,
            "updated_by": "test-script"
        }')
    check_response "$response" "200" "UPDATE mapping (UUID: $uuid)"
else
    echo -e "${YELLOW}⊘${NC} UPDATE mapping (skipped, no UUID from CREATE)"
fi

echo ""
echo "5. Test soft DELETE mapping"
if [ -n "$uuid" ]; then
    response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X DELETE \
        "$BASE_URL/admin/api-key-priority/mappings/$uuid" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json")
    check_response "$response" "200" "DELETE mapping (soft delete, UUID: $uuid)"
else
    echo -e "${YELLOW}⊘${NC} DELETE mapping (skipped, no UUID from CREATE)"
fi

echo ""
echo "6. Test LIST after delete (should return empty array - soft deleted records excluded)"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X GET \
    "$BASE_URL/admin/api-key-priority/mappings" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json")
check_response "$response" "200" "LIST after delete (should be empty)"

echo ""
echo "7. Test reload cache"
response=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST \
    "$BASE_URL/admin/api-key-priority/reload" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json")
check_response "$response" "200" "Reload cache"

echo ""
echo "=== Test Summary ==="
echo -e "Passed: ${GREEN}$test_passed${NC}"
echo -e "Failed: ${RED}$test_failed${NC}"
echo ""

if [ $test_failed -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
