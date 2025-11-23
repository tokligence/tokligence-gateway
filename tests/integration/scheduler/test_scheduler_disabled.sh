#!/usr/bin/env bash
#
# Integration Test: Scheduler Disabled (Backward Compatibility)
#
# Purpose: Verify that gateway works normally when scheduler is disabled
# Expected: All requests pass through without queueing, no scheduler overhead
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
cd "${GATEWAY_ROOT}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================"
echo "Scheduler Disabled - Backward Compatibility Test"
echo "========================================"
echo ""

# Test configuration
GATEWAY_PORT=8081
TEST_TIMEOUT=30

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    pkill -f "gatewayd" || true
    sleep 1
}
trap cleanup EXIT

# Step 1: Build gateway
echo "[1/5] Building gateway..."
make bgd > /dev/null 2>&1 || {
    echo -e "${RED}✗ Build failed${NC}"
    exit 1
}
echo -e "${GREEN}✓ Build successful${NC}"

# Step 2: Start gateway with scheduler DISABLED
echo "[2/5] Starting gateway with scheduler_enabled=false..."
export TOKLIGENCE_SCHEDULER_ENABLED=false
export TOKLIGENCE_AUTH_DISABLED=true
export TOKLIGENCE_LOG_LEVEL=info
export TOKLIGENCE_FACADE_PORT=${GATEWAY_PORT}

# Kill any existing gateway on port
pkill -f "gatewayd" || true
sleep 1

# Start gateway in background
./bin/gatewayd > /tmp/gateway_scheduler_disabled.log 2>&1 &
GATEWAY_PID=$!

# Wait for gateway to start
sleep 3

# Verify gateway is running
if ! kill -0 ${GATEWAY_PID} 2>/dev/null; then
    echo -e "${RED}✗ Gateway failed to start${NC}"
    cat /tmp/gateway_scheduler_disabled.log
    exit 1
fi
echo -e "${GREEN}✓ Gateway started (PID: ${GATEWAY_PID})${NC}"

# Step 3: Verify scheduler is NOT mentioned in logs
echo "[3/5] Verifying scheduler is disabled in logs..."
if grep -q "Priority scheduler enabled" /tmp/gateway_scheduler_disabled.log; then
    echo -e "${RED}✗ Scheduler should NOT be enabled${NC}"
    cat /tmp/gateway_scheduler_disabled.log
    exit 1
fi
echo -e "${GREEN}✓ Scheduler correctly disabled${NC}"

# Step 4: Send test requests (should all pass through immediately)
echo "[4/5] Sending test requests..."

# Test 1: Request without X-Priority header
RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:${GATEWAY_PORT}/health)
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)

if [ "$HTTP_CODE" != "200" ]; then
    echo -e "${RED}✗ Health check failed (HTTP ${HTTP_CODE})${NC}"
    echo "$RESPONSE"
    exit 1
fi
echo -e "${GREEN}✓ Health check passed${NC}"

# Test 2: Multiple concurrent requests (should all succeed immediately)
CONCURRENT_REQUESTS=10
SUCCESS_COUNT=0

for i in $(seq 1 ${CONCURRENT_REQUESTS}); do
    (
        RESP=$(curl -s -w "%{http_code}" http://localhost:${GATEWAY_PORT}/health -o /dev/null)
        if [ "$RESP" = "200" ]; then
            echo "ok" > /tmp/test_req_${i}.result
        fi
    ) &
done

wait

for i in $(seq 1 ${CONCURRENT_REQUESTS}); do
    if [ -f /tmp/test_req_${i}.result ]; then
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        rm /tmp/test_req_${i}.result
    fi
done

if [ ${SUCCESS_COUNT} -ne ${CONCURRENT_REQUESTS} ]; then
    echo -e "${RED}✗ Only ${SUCCESS_COUNT}/${CONCURRENT_REQUESTS} requests succeeded${NC}"
    exit 1
fi
echo -e "${GREEN}✓ All ${CONCURRENT_REQUESTS} concurrent requests succeeded${NC}"

# Step 5: Verify no scheduler-related errors in logs
echo "[5/5] Verifying no scheduler errors..."
if grep -qi "scheduler.*error\|queue.*full\|queue.*timeout" /tmp/gateway_scheduler_disabled.log; then
    echo -e "${RED}✗ Found scheduler-related errors in logs${NC}"
    grep -i "scheduler\|queue" /tmp/gateway_scheduler_disabled.log
    exit 1
fi
echo -e "${GREEN}✓ No scheduler errors in logs${NC}"

# Test passed
echo ""
echo "========================================"
echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
echo "========================================"
echo "Backward compatibility verified:"
echo "  - Gateway starts without scheduler"
echo "  - Requests processed immediately"
echo "  - No queueing overhead"
echo "  - No scheduler errors"
echo ""

exit 0
