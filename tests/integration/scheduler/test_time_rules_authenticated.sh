#!/usr/bin/env bash
#
# Integration Test: Time-Based Rules - Authenticated Mode
#
# Tests time rules endpoints with real authentication enabled.
# Verifies permission boundaries between root_admin and regular users.
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
cd "$PROJECT_ROOT"

# Port configuration
PORT_OFFSET=${PORT_OFFSET:-0}
export TOKLIGENCE_FACADE_PORT=$((8081 + PORT_OFFSET))
GATEWAY_PORT=${TOKLIGENCE_FACADE_PORT:-8081}

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    pkill -f gatewayd || true
    sleep 1
}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${BOLD}=== Integration Test: Time-Based Rules - Authenticated Mode ===${NC}"
echo

# Cleanup any previous instances
cleanup

# Remove old test databases
rm -f /tmp/tokligence_identity_auth_test.db
rm -f /tmp/tokligence_ledger_auth_test.db

# Create test config
cat > /tmp/test_time_rules_auth.ini << 'EOF'
[time_rules]
enabled = true
timezone = UTC

[[time_rules.weight_adjustment]]
name = Admin Test Rule
priorities = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
weights = [100, 50, 25, 10, 5, 3, 2, 1, 1, 1]
start_hour = 0
end_hour = 23
days_of_week = [0, 1, 2, 3, 4, 5, 6]
description = Test rule for authenticated access
EOF

echo -e "${GREEN}✓ Created test config: /tmp/test_time_rules_auth.ini${NC}"

# Start gatewayd with authentication ENABLED
export TOKLIGENCE_TIME_RULES_ENABLED=true
export TOKLIGENCE_TIME_RULES_CONFIG=/tmp/test_time_rules_auth.ini
export TOKLIGENCE_LOG_LEVEL=info
export TOKLIGENCE_MARKETPLACE_ENABLED=false
export TOKLIGENCE_SCHEDULER_ENABLED=true
export TOKLIGENCE_ADMIN_PORT=0
export TOKLIGENCE_OPENAI_PORT=0
export TOKLIGENCE_ANTHROPIC_PORT=0
export TOKLIGENCE_GEMINI_PORT=0
export TOKLIGENCE_MULTIPORT_MODE=false
export TOKLIGENCE_ENABLE_FACADE=true
export TOKLIGENCE_IDENTITY_PATH=/tmp/tokligence_identity_auth_test.db
export TOKLIGENCE_LEDGER_PATH=/tmp/tokligence_ledger_auth_test.db
export TOKLIGENCE_AUTH_DISABLED=false  # IMPORTANT: Enable authentication

echo "Starting gatewayd with authentication enabled..."
./bin/gatewayd > /tmp/gatewayd_time_rules_auth.log 2>&1 &
GATEWAYD_PID=$!

# Wait for gateway to start
sleep 3
if ! ps -p $GATEWAYD_PID > /dev/null; then
    echo -e "${RED}✗ ERROR: gatewayd failed to start${NC}"
    echo "Log output:"
    tail -20 /tmp/gatewayd_time_rules_auth.log
    exit 1
fi
echo -e "${GREEN}✓ Started gatewayd (PID: $GATEWAYD_PID)${NC}"

# Wait for HTTP server to be ready
GATEWAY_READY=false
for i in {1..15}; do
    if curl -s http://localhost:$GATEWAY_PORT/health > /dev/null 2>&1; then
        GATEWAY_READY=true
        break
    fi
    sleep 1
done

if [ "$GATEWAY_READY" = false ]; then
    echo -e "${RED}✗ ERROR: Gateway health check failed after 15 seconds${NC}"
    echo "Gateway log:"
    tail -20 /tmp/gatewayd_time_rules_auth.log
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Gateway is running${NC}"

# Setup authentication: Create admin user and regular user
echo
echo "Setting up test users..."
go build -o /tmp/setup_test_auth tests/integration/scheduler/setup_test_auth.go
ADMIN_TOKEN=$(/tmp/setup_test_auth /tmp/tokligence_identity_auth_test.db admin@test.dev admin)
if [ -z "$ADMIN_TOKEN" ]; then
    echo -e "${RED}✗ ERROR: Failed to create admin user${NC}"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Created admin user and API key${NC}"

# Create a regular (non-admin) user
USER_TOKEN=$(/tmp/setup_test_auth /tmp/tokligence_identity_auth_test.db user@test.dev consumer)
if [ -z "$USER_TOKEN" ]; then
    echo -e "${RED}✗ ERROR: Failed to create regular user${NC}"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Created regular user and API key${NC}"

echo
echo -e "${BOLD}========================================${NC}"
echo -e "${BOLD}Test 6.4: Root Admin Access to Time Rules${NC}"
echo -e "${BOLD}========================================${NC}"
echo

# Test 1: Admin can access GET /admin/time-rules/status
echo "Test 1: Admin GET /admin/time-rules/status"
echo "-------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:$GATEWAY_PORT/admin/time-rules/status \
    -H "Authorization: Bearer $ADMIN_TOKEN")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 200, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Admin can access time-rules status (HTTP 200)${NC}"

# Verify response contains expected fields
if ! echo "$BODY" | grep -q '"enabled"'; then
    echo -e "${RED}✗ FAILED: Response missing 'enabled' field${NC}"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Response contains expected fields${NC}"

# Test 2: Admin can POST /admin/time-rules/apply
echo
echo "Test 2: Admin POST /admin/time-rules/apply"
echo "-------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:$GATEWAY_PORT/admin/time-rules/apply \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 200, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Admin can apply time rules (HTTP 200)${NC}"

# Test 3: Admin can POST /admin/time-rules/reload
echo
echo "Test 3: Admin POST /admin/time-rules/reload"
echo "-------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:$GATEWAY_PORT/admin/time-rules/reload \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "Content-Type: application/json")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 200, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Admin can reload config (HTTP 200)${NC}"

echo
echo -e "${GREEN}${BOLD}=== Test 6.4 PASSED: Admin has full access ===${NC}"

echo
echo -e "${BOLD}========================================${NC}"
echo -e "${BOLD}Test 6.5: Regular User Access Denied${NC}"
echo -e "${BOLD}========================================${NC}"
echo

# Test 4: Regular user CANNOT access GET /admin/time-rules/status
echo "Test 4: Regular user GET /admin/time-rules/status"
echo "--------------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:$GATEWAY_PORT/admin/time-rules/status \
    -H "Authorization: Bearer $USER_TOKEN")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "403" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 403, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Regular user denied access (HTTP 403)${NC}"

# Verify error message
if ! echo "$BODY" | grep -q "admin access required"; then
    echo -e "${RED}✗ FAILED: Expected 'admin access required' error message${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Correct error message returned${NC}"

# Test 5: Regular user CANNOT POST /admin/time-rules/apply
echo
echo "Test 5: Regular user POST /admin/time-rules/apply"
echo "--------------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:$GATEWAY_PORT/admin/time-rules/apply \
    -H "Authorization: Bearer $USER_TOKEN" \
    -H "Content-Type: application/json")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "403" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 403, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Regular user denied apply access (HTTP 403)${NC}"

# Test 6: Regular user CANNOT POST /admin/time-rules/reload
echo
echo "Test 6: Regular user POST /admin/time-rules/reload"
echo "--------------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:$GATEWAY_PORT/admin/time-rules/reload \
    -H "Authorization: Bearer $USER_TOKEN" \
    -H "Content-Type: application/json")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "403" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 403, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Regular user denied reload access (HTTP 403)${NC}"

echo
echo -e "${GREEN}${BOLD}=== Test 6.5 PASSED: Regular users properly denied ===${NC}"

echo
echo -e "${BOLD}========================================${NC}"
echo -e "${BOLD}Test 6.6: Invalid/Missing Token${NC}"
echo -e "${BOLD}========================================${NC}"
echo

# Test 7: No token provided
echo "Test 7: Request without Authorization header"
echo "--------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:$GATEWAY_PORT/admin/time-rules/status)
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "401" ] && [ "$HTTP_CODE" != "403" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 401 or 403, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Request without token rejected (HTTP $HTTP_CODE)${NC}"

# Test 8: Invalid token
echo
echo "Test 8: Request with invalid token"
echo "-----------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:$GATEWAY_PORT/admin/time-rules/status \
    -H "Authorization: Bearer invalid_token_12345")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "401" ] && [ "$HTTP_CODE" != "403" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 401 or 403, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Invalid token rejected (HTTP $HTTP_CODE)${NC}"

# Test 9: Malformed Authorization header
echo
echo "Test 9: Request with malformed Authorization header"
echo "----------------------------------------------------"
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:$GATEWAY_PORT/admin/time-rules/status \
    -H "Authorization: InvalidFormat")
BODY=$(echo "$RESPONSE" | head -n -1)
HTTP_CODE=$(echo "$RESPONSE" | tail -1)

if [ "$HTTP_CODE" != "401" ] && [ "$HTTP_CODE" != "403" ]; then
    echo -e "${RED}✗ FAILED: Expected HTTP 401 or 403, got $HTTP_CODE${NC}"
    echo "Response: $BODY"
    cleanup
    exit 1
fi
echo -e "${GREEN}✓ Malformed header rejected (HTTP $HTTP_CODE)${NC}"

echo
echo -e "${GREEN}${BOLD}=== Test 6.6 PASSED: Invalid tokens properly rejected ===${NC}"

# Cleanup
cleanup

echo
echo -e "${GREEN}${BOLD}========================================${NC}"
echo -e "${GREEN}${BOLD}All Authenticated Tests Passed!${NC}"
echo -e "${GREEN}${BOLD}========================================${NC}"
echo -e "${GREEN}✓ Root admin has full time-rules access${NC}"
echo -e "${GREEN}✓ Regular users properly denied access${NC}"
echo -e "${GREEN}✓ Invalid/missing tokens properly rejected${NC}"
echo
