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

# Test configuration (base ports + optional offset to avoid conflicts)
PORT_OFFSET=${PORT_OFFSET:-10000}
GATEWAY_PORT=$((8081 + PORT_OFFSET))
ADMIN_PORT=$((8079 + PORT_OFFSET))
OPENAI_PORT=$((8082 + PORT_OFFSET))
ANTHROPIC_PORT=$((8083 + PORT_OFFSET))
GEMINI_PORT=$((8084 + PORT_OFFSET))
TEST_TIMEOUT=30
CURL_OPTS="--max-time 5 --connect-timeout 2"
export TOKLIGENCE_IDENTITY_PATH=${TOKLIGENCE_IDENTITY_PATH:-/tmp/tokligence_identity.db}
export TOKLIGENCE_LEDGER_PATH=${TOKLIGENCE_LEDGER_PATH:-/tmp/tokligence_ledger.db}
export TOKLIGENCE_MODEL_METADATA_URL=""
export TOKLIGENCE_MODEL_METADATA_FILE=${TOKLIGENCE_MODEL_METADATA_FILE:-data/model_metadata.json}

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    make gdx > /dev/null 2>&1 || true
    pkill -f "gatewayd" > /dev/null 2>&1 || true
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
export TOKLIGENCE_ADMIN_PORT=0
export TOKLIGENCE_OPENAI_PORT=0
export TOKLIGENCE_ANTHROPIC_PORT=0
export TOKLIGENCE_GEMINI_PORT=0
export TOKLIGENCE_MARKETPLACE_ENABLED=false
export TOKLIGENCE_MULTIPORT_MODE=false
export TOKLIGENCE_ENABLE_FACADE=true

# Stop any existing gateway cleanly
make gdx > /dev/null 2>&1 || true
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
RESPONSE=$(curl -s ${CURL_OPTS} -w "\n%{http_code}" http://localhost:${GATEWAY_PORT}/health)
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

# Clean up old result files
rm -f /tmp/test_req_*.result

for i in $(seq 1 ${CONCURRENT_REQUESTS}); do
    (
        RESP=$(curl -s ${CURL_OPTS} -w "%{http_code}" http://localhost:${GATEWAY_PORT}/health -o /dev/null 2>/dev/null)
        if [ "$RESP" = "200" ]; then
            echo "ok" > /tmp/test_req_${i}.result
        fi
    ) &
done

# Wait with timeout (15 seconds should be plenty for 10 requests with 5s timeout each)
for attempt in $(seq 1 30); do
    RUNNING=$(jobs -r | wc -l)
    if [ "$RUNNING" -eq 0 ]; then
        break
    fi
    sleep 0.5
done

# Kill any remaining background jobs
jobs -p | xargs -r kill -9 2>/dev/null || true
wait 2>/dev/null || true

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
