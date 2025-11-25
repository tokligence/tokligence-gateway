#!/bin/bash
# GraphQL API Integration Test Script
# Usage: ./scripts/graphql_test.sh [endpoint]
# Default endpoint: http://localhost:8081/graphql

set -e

ENDPOINT="${1:-http://localhost:8081/graphql}"
VERBOSE="${VERBOSE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_pass() { echo -e "${GREEN}[PASS]${NC} $1"; }
log_fail() { echo -e "${RED}[FAIL]${NC} $1"; exit 1; }
log_info() { echo -e "${YELLOW}[INFO]${NC} $1"; }

# GraphQL request helper
gql_request() {
    local query="$1"
    local variables="${2:-{}}"

    local response
    response=$(curl -s -X POST "$ENDPOINT" \
        -H "Content-Type: application/json" \
        -d "{\"query\": \"$query\", \"variables\": $variables}")

    if [ "$VERBOSE" = "true" ]; then
        echo "$response" | jq .
    fi

    echo "$response"
}

# Check for errors in response
check_no_errors() {
    local response="$1"
    local test_name="$2"

    if echo "$response" | jq -e '.errors' > /dev/null 2>&1; then
        log_fail "$test_name: $(echo "$response" | jq -r '.errors[0].message')"
    fi
    log_pass "$test_name"
}

log_info "Testing GraphQL API at $ENDPOINT"
echo "=============================================="

# Test 1: Ping
log_info "Test 1: Ping query"
RESPONSE=$(gql_request "query { ping }")
if echo "$RESPONSE" | jq -e '.data.ping == "pong"' > /dev/null 2>&1; then
    log_pass "Ping returns 'pong'"
else
    log_fail "Ping test failed"
fi

# Test 2: Create User
log_info "Test 2: Create User mutation"
UNIQUE_EMAIL="graphql-test-$(date +%s)@example.com"
RESPONSE=$(gql_request "mutation CreateUser(\$input: CreateUserInput!) { createUser(input: \$input) { id email role displayName status } }" \
    "{\"input\": {\"email\": \"$UNIQUE_EMAIL\", \"role\": \"USER\", \"displayName\": \"GraphQL Test User\"}}")

if echo "$RESPONSE" | jq -e '.data.createUser.id' > /dev/null 2>&1; then
    USER_ID=$(echo "$RESPONSE" | jq -r '.data.createUser.id')
    log_pass "Created user with ID: $USER_ID"
else
    log_fail "Create user failed: $(echo "$RESPONSE" | jq -r '.errors[0].message // .data')"
fi

# Test 3: Get User by ID
log_info "Test 3: Get User by ID query"
RESPONSE=$(gql_request "query GetUser(\$id: ID!) { user(id: \$id) { id email displayName } }" \
    "{\"id\": \"$USER_ID\"}")

if echo "$RESPONSE" | jq -e ".data.user.id == \"$USER_ID\"" > /dev/null 2>&1; then
    log_pass "Retrieved user by ID"
else
    log_fail "Get user by ID failed"
fi

# Test 4: Get User by Email
log_info "Test 4: Get User by Email query"
RESPONSE=$(gql_request "query GetUserByEmail(\$email: String!) { userByEmail(email: \$email) { id email } }" \
    "{\"email\": \"$UNIQUE_EMAIL\"}")

if echo "$RESPONSE" | jq -e ".data.userByEmail.email == \"$UNIQUE_EMAIL\"" > /dev/null 2>&1; then
    log_pass "Retrieved user by email"
else
    log_fail "Get user by email failed"
fi

# Test 5: Update User
log_info "Test 5: Update User mutation"
RESPONSE=$(gql_request "mutation UpdateUser(\$id: ID!, \$input: UpdateUserInput!) { updateUser(id: \$id, input: \$input) { id displayName } }" \
    "{\"id\": \"$USER_ID\", \"input\": {\"displayName\": \"Updated Name\"}}")

if echo "$RESPONSE" | jq -e '.data.updateUser.displayName == "Updated Name"' > /dev/null 2>&1; then
    log_pass "Updated user displayName"
else
    log_fail "Update user failed"
fi

# Test 6: List Users
log_info "Test 6: List Users query"
RESPONSE=$(gql_request "query { users { id email role } }")

if echo "$RESPONSE" | jq -e '.data.users | length > 0' > /dev/null 2>&1; then
    USER_COUNT=$(echo "$RESPONSE" | jq '.data.users | length')
    log_pass "Listed $USER_COUNT users"
else
    log_fail "List users failed"
fi

# Test 7: Delete User
log_info "Test 7: Delete User mutation"
RESPONSE=$(gql_request "mutation DeleteUser(\$id: ID!) { deleteUser(id: \$id) }" \
    "{\"id\": \"$USER_ID\"}")

if echo "$RESPONSE" | jq -e '.data.deleteUser == true' > /dev/null 2>&1; then
    log_pass "Deleted user"
else
    log_fail "Delete user failed"
fi

# Test 8: Verify Deletion
log_info "Test 8: Verify user deletion"
RESPONSE=$(gql_request "query GetUser(\$id: ID!) { user(id: \$id) { id } }" \
    "{\"id\": \"$USER_ID\"}")

if echo "$RESPONSE" | jq -e '.data.user == null' > /dev/null 2>&1; then
    log_pass "User successfully deleted (returns null)"
else
    log_fail "User still exists after deletion"
fi

echo "=============================================="
log_info "All GraphQL API tests passed!"
