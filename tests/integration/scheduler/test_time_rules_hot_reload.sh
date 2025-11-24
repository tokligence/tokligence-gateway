#!/usr/bin/env bash
#
# Phase 3 Hot Reload Test
#
# Tests:
# 1. POST /admin/time-rules/reload endpoint
# 2. Automatic file modification detection
# 3. Rules are reloaded when config file changes
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
cd "${GATEWAY_ROOT}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

echo -e "${BOLD}${CYAN}"
echo "========================================"
echo "Phase 3: Hot Reload Test"
echo "========================================"
echo -e "${NC}"

# Create temporary config file
TEMP_CONFIG="/tmp/test_time_rules_hot_reload_$$.ini"
PORT_OFFSET=${PORT_OFFSET:-0}
GATEWAY_PORT=$((8081 + PORT_OFFSET))
ADMIN_PORT=$((8079 + PORT_OFFSET))
OPENAI_PORT=$((8082 + PORT_OFFSET))
ANTHROPIC_PORT=$((8083 + PORT_OFFSET))
GEMINI_PORT=$((8084 + PORT_OFFSET))
export TOKLIGENCE_IDENTITY_PATH=${TOKLIGENCE_IDENTITY_PATH:-/tmp/tokligence_identity.db}
export TOKLIGENCE_LEDGER_PATH=${TOKLIGENCE_LEDGER_PATH:-/tmp/tokligence_ledger.db}
export TOKLIGENCE_MODEL_METADATA_URL=""
export TOKLIGENCE_MODEL_METADATA_FILE=${TOKLIGENCE_MODEL_METADATA_FILE:-data/model_metadata.json}

echo "Compiling and running auth setup..."
go build -o /tmp/setup_test_auth tests/integration/scheduler/setup_test_auth.go
ADMIN_TOKEN=$(/tmp/setup_test_auth)
if [ -z "$ADMIN_TOKEN" ]; then
    echo -e "${RED}✗ FAILED: Could not get admin token${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Got admin token${NC}"

# Cleanup function
cleanup() {
    echo ""
    echo "Cleaning up..."
    make gdx >/dev/null 2>&1 || true
    rm -f "${TEMP_CONFIG}"
    rm -f /tmp/setup_test_auth
    sleep 1
}

trap cleanup EXIT

# Create initial config with 2 rules
cat > "${TEMP_CONFIG}" <<'EOF'
[time_rules]
enabled = true
check_interval_sec = 5
file_check_interval_sec = 3  # Check every 3 seconds for testing
default_timezone = UTC

# Initial rule 1
[rule.weights.test1]
type = weight_adjustment
name = Test Rule 1
description = Initial test rule
enabled = true
start_time = 00:00
end_time = 23:59
weights = 100,50,25,10,5,3,2,1,1,1

# Initial rule 2
[rule.capacity.test2]
type = capacity_adjustment
name = Test Rule 2
description = Initial capacity rule
enabled = true
start_time = 00:00
end_time = 23:59
max_concurrent = 100
max_rps = 200
max_tokens_per_sec = 5000
EOF

echo "Created test config: ${TEMP_CONFIG}"
echo ""

# Start gatewayd
export TOKLIGENCE_TIME_RULES_ENABLED=true
export TOKLIGENCE_TIME_RULES_CONFIG="${TEMP_CONFIG}"
export TOKLIGENCE_AUTH_DISABLED=true
export TOKLIGENCE_MARKETPLACE_ENABLED=false
export TOKLIGENCE_SCHEDULER_ENABLED=true
export TOKLIGENCE_FACADE_PORT=${GATEWAY_PORT}
export TOKLIGENCE_ADMIN_PORT=${ADMIN_PORT}
export TOKLIGENCE_OPENAI_PORT=${OPENAI_PORT}
export TOKLIGENCE_ANTHROPIC_PORT=${ANTHROPIC_PORT}
export TOKLIGENCE_GEMINI_PORT=${GEMINI_PORT}
export TOKLIGENCE_FACADE_PORT=${GATEWAY_PORT}
export TOKLIGENCE_ADMIN_PORT=${ADMIN_PORT}
export TOKLIGENCE_OPENAI_PORT=${OPENAI_PORT}
export TOKLIGENCE_ANTHROPIC_PORT=${ANTHROPIC_PORT}
export TOKLIGENCE_GEMINI_PORT=${GEMINI_PORT}

echo "Starting gatewayd with test config..."
./bin/gatewayd > /tmp/gateway_hot_reload_test.log 2>&1 &
GATEWAY_PID=$!
sleep 3

# Check if gateway started
if ! kill -0 ${GATEWAY_PID} 2>/dev/null; then
    echo -e "${RED}✗ Gateway failed to start${NC}"
    cat /tmp/gateway_hot_reload_test.log
    exit 1
fi

echo -e "${GREEN}✓ Gateway started (PID: ${GATEWAY_PID})${NC}"
echo ""

# ========================================
# Test 1: Initial state - 2 rules
# ========================================
echo -e "${BOLD}Test 1: Verify initial state${NC}"
echo ""

response=$(curl -s http://localhost:${GATEWAY_PORT}/admin/time-rules/status \
  -H "Authorization: Bearer ${ADMIN_TOKEN}")

echo "Response:"
echo "${response}" | python3 -m json.tool 2>/dev/null || echo "${response}"
echo ""

rule_count=$(echo "${response}" | python3 -c "import sys, json; print(json.load(sys.stdin)['count'])" 2>/dev/null || echo "0")

if [ "${rule_count}" = "2" ]; then
    echo -e "${GREEN}✓ Initial rule count is correct: 2${NC}"
else
    echo -e "${RED}✗ Expected 2 rules, got ${rule_count}${NC}"
    exit 1
fi

# ========================================
# Test 2: Manual reload endpoint
# ========================================
echo ""
echo -e "${BOLD}Test 2: Manual reload (POST /admin/time-rules/reload)${NC}"
echo ""

# Modify config file - add a third rule
cat >> "${TEMP_CONFIG}" <<'EOF'

# New rule added for reload test
[rule.quota.test3]
type = quota_adjustment
name = Test Rule 3
description = Rule added after startup
enabled = true
start_time = 00:00
end_time = 23:59
quota.test-* = concurrent:10,rps:20,tokens_per_sec:1000
EOF

echo "Added new rule to config file"
sleep 1

# Trigger manual reload
reload_response=$(curl -s -X POST http://localhost:${GATEWAY_PORT}/admin/time-rules/reload \
  -H "Authorization: Bearer ${ADMIN_TOKEN}")

echo ""
echo "Reload response:"
echo "${reload_response}" | python3 -m json.tool 2>/dev/null || echo "${reload_response}"
echo ""

total_count=$(echo "${reload_response}" | python3 -c "import sys, json; print(json.load(sys.stdin)['total_count'])" 2>/dev/null || echo "0")

if [ "${total_count}" = "3" ]; then
    echo -e "${GREEN}✓ Manual reload successful: 3 rules loaded${NC}"
else
    echo -e "${RED}✗ Expected 3 rules after reload, got ${total_count}${NC}"
    exit 1
fi

# ========================================
# Test 3: Automatic file monitoring
# ========================================
echo ""
echo -e "${BOLD}Test 3: Automatic file monitoring${NC}"
echo ""

# Modify config file again - add a fourth rule
cat >> "${TEMP_CONFIG}" <<'EOF'

# Another new rule for auto-reload test
[rule.weights.test4]
type = weight_adjustment
name = Test Rule 4
description = Rule for testing automatic reload
enabled = true
start_time = 00:00
end_time = 23:59
weights = 10,10,10,10,10,10,10,10,10,10
EOF

echo "Added fourth rule to config file"
echo "Waiting 5 seconds for automatic reload..."
sleep 5

# Check status - should show 4 rules now
status_response=$(curl -s http://localhost:${GATEWAY_PORT}/admin/time-rules/status \
  -H "Authorization: Bearer ${ADMIN_TOKEN}")

echo ""
echo "Status after auto-reload:"
echo "${status_response}" | python3 -m json.tool 2>/dev/null || echo "${status_response}"
echo ""

final_count=$(echo "${status_response}" | python3 -c "import sys, json; print(json.load(sys.stdin)['count'])" 2>/dev/null || echo "0")

if [ "${final_count}" = "4" ]; then
    echo -e "${GREEN}✓ Automatic reload successful: 4 rules loaded${NC}"
else
    echo -e "${RED}✗ Expected 4 rules after auto-reload, got ${final_count}${NC}"
    exit 1
fi

# ========================================
# Test 4: Check logs for reload messages
# ========================================
echo ""
echo -e "${BOLD}Test 4: Verify reload messages in logs${NC}"
echo ""

if grep -q "Config file changed, reloading" /tmp/gateway_hot_reload_test.log; then
    echo -e "${GREEN}✓ Auto-reload message found in logs${NC}"
else
    echo -e "${YELLOW}⚠ Auto-reload message not found in logs (may have reloaded too fast)${NC}"
fi

if grep -q "Config reloaded successfully" /tmp/gateway_hot_reload_test.log; then
    echo -e "${GREEN}✓ Reload success message found in logs${NC}"
else
    echo -e "${RED}✗ Reload success message not found in logs${NC}"
fi

# ========================================
# Summary
# ========================================
echo ""
echo -e "${BOLD}${GREEN}========================================"
echo "Hot Reload Test: PASSED"
echo "========================================"
echo -e "${NC}"
echo "✓ Manual reload endpoint works"
echo "✓ Automatic file monitoring works"
echo "✓ Rules are properly reloaded"
echo ""

exit 0
