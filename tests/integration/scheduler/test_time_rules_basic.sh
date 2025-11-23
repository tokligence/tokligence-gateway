#!/bin/bash
# Integration test for Phase 3: Time-Based Dynamic Rules - Basic Functionality
#
# Tests:
# 1. Rule engine starts with config file
# 2. Rules are loaded correctly
# 3. HTTP endpoints work
# 4. Rules are evaluated based on current time

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== Integration Test: Time-Based Rules - Basic Functionality ==="
echo

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    pkill -f gatewayd || true
    rm -f /tmp/test_time_rules.ini
    rm -f /tmp/gatewayd_time_rules.log
}

trap cleanup EXIT

# Build if needed
if [ ! -f "bin/gatewayd" ]; then
    echo "Building gatewayd..."
    make bgd
fi

# Create test config with only one simple rule
cat > /tmp/test_time_rules.ini <<'EOF'
[time_rules]
enabled = true
check_interval_sec = 5
default_timezone = UTC

[rule.weights.test]
type = weight_adjustment
name = Test Weight Rule
description = Test rule for integration test
enabled = true
start_time = 00:00
end_time = 23:59
weights = 100,50,25,10,5,3,2,1,1,1

[rule.quota.test]
type = quota_adjustment
name = Test Quota Rule
description = Test quota adjustment
enabled = true
start_time = 00:00
end_time = 23:59
quota.test-* = concurrent:10,rps:20,tokens_per_sec:1000

[rule.capacity.test]
type = capacity_adjustment
name = Test Capacity Rule
description = Test capacity adjustment
enabled = true
start_time = 00:00
end_time = 23:59
max_concurrent = 100
max_rps = 200
max_tokens_per_sec = 5000
EOF

echo "✓ Created test config: /tmp/test_time_rules.ini"

# Start gatewayd with time rules
export TOKLIGENCE_TIME_RULES_ENABLED=true
export TOKLIGENCE_TIME_RULES_CONFIG=/tmp/test_time_rules.ini
export TOKLIGENCE_LOG_LEVEL=info
export TOKLIGENCE_AUTH_DISABLED=true
export TOKLIGENCE_MARKETPLACE_ENABLED=false
export TOKLIGENCE_SCHEDULER_ENABLED=true

echo "Starting gatewayd..."
./bin/gatewayd > /tmp/gatewayd_time_rules.log 2>&1 &
GATEWAYD_PID=$!
echo "✓ Started gatewayd (PID: $GATEWAYD_PID)"

# Wait for startup
sleep 3

# Verify gatewayd is running
if ! ps -p $GATEWAYD_PID > /dev/null; then
    echo "✗ ERROR: gatewayd failed to start"
    echo "Log output:"
    cat /tmp/gatewayd_time_rules.log
    exit 1
fi
echo "✓ Gateway is running"

# Test 1: GET /admin/time-rules/status
echo
echo "Test 1: GET /admin/time-rules/status"
echo "---------------------------------------"
RESPONSE=$(curl -s -X GET http://localhost:8081/admin/time-rules/status \
    -H "Authorization: Bearer test" \
    -w "\n%{http_code}")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)

if [ "$HTTP_CODE" != "200" ]; then
    echo "✗ FAILED: Expected HTTP 200, got $HTTP_CODE"
    echo "Response: $BODY"
    exit 1
fi

echo "✓ HTTP 200 OK"

# Verify response structure
ENABLED=$(echo "$BODY" | grep -o '"enabled":\s*true')
if [ -z "$ENABLED" ]; then
    echo "✗ FAILED: Rule engine not enabled"
    echo "Response: $BODY"
    exit 1
fi
echo "✓ Rule engine is enabled"

# Verify rule count
RULE_COUNT=$(echo "$BODY" | grep -o '"count":\s*[0-9]*' | grep -o '[0-9]*')
if [ "$RULE_COUNT" != "3" ]; then
    echo "✗ FAILED: Expected 3 rules, got $RULE_COUNT"
    exit 1
fi
echo "✓ Rule count is correct: $RULE_COUNT"

# Verify all rules are active (since they run 00:00-23:59)
ACTIVE_COUNT=$(echo "$BODY" | grep -o '"active":\s*true' | wc -l)
if [ "$ACTIVE_COUNT" != "3" ]; then
    echo "✗ FAILED: Expected 3 active rules, got $ACTIVE_COUNT"
    echo "Response: $BODY"
    exit 1
fi
echo "✓ All 3 rules are active"

# Test 2: POST /admin/time-rules/apply
echo
echo "Test 2: POST /admin/time-rules/apply"
echo "-------------------------------------"
RESPONSE=$(curl -s -X POST http://localhost:8081/admin/time-rules/apply \
    -H "Authorization: Bearer test" \
    -w "\n%{http_code}")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)

if [ "$HTTP_CODE" != "200" ]; then
    echo "✗ FAILED: Expected HTTP 200, got $HTTP_CODE"
    echo "Response: $BODY"
    exit 1
fi
echo "✓ HTTP 200 OK"

# Verify message
MESSAGE=$(echo "$BODY" | grep -o '"message":\s*"Rules applied successfully"')
if [ -z "$MESSAGE" ]; then
    echo "✗ FAILED: Missing success message"
    echo "Response: $BODY"
    exit 1
fi
echo "✓ Rules applied successfully"

# Verify active_count
ACTIVE_COUNT=$(echo "$BODY" | grep -o '"active_count":\s*[0-9]*' | grep -o '[0-9]*')
if [ "$ACTIVE_COUNT" != "3" ]; then
    echo "✗ FAILED: Expected 3 active rules, got $ACTIVE_COUNT"
    exit 1
fi
echo "✓ Active rule count is correct: $ACTIVE_COUNT"

# Test 3: Verify logs
echo
echo "Test 3: Verify logs"
echo "--------------------"
if ! grep -q "Time-based rule engine started successfully" /tmp/gatewayd_time_rules.log; then
    echo "✗ FAILED: Engine startup message not found in logs"
    exit 1
fi
echo "✓ Engine startup logged"

if ! grep -q "RuleEngine: Starting" /tmp/gatewayd_time_rules.log; then
    echo "✗ FAILED: Engine start message not found in logs"
    exit 1
fi
echo "✓ Engine start logged"

if ! grep -q "Applying weight rule" /tmp/gatewayd_time_rules.log; then
    echo "✗ FAILED: Weight rule application not found in logs"
    exit 1
fi
echo "✓ Weight rule application logged"

if ! grep -q "Applying quota rule" /tmp/gatewayd_time_rules.log; then
    echo "✗ FAILED: Quota rule application not found in logs"
    exit 1
fi
echo "✓ Quota rule application logged"

if ! grep -q "Applying capacity rule" /tmp/gatewayd_time_rules.log; then
    echo "✗ FAILED: Capacity rule application not found in logs"
    exit 1
fi
echo "✓ Capacity rule application logged"

echo
echo "=== All tests passed! ==="
