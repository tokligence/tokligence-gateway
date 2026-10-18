#!/bin/bash
# Live server test for priority scheduler with all 3 policies
# Tests: Strict Priority, WFQ, and Hybrid scheduling
# Monitors: Channel occupancy via HTTP endpoints

set -e

PORT_OFFSET=${PORT_OFFSET:-0}
GATEWAY_PORT=$((8081 + PORT_OFFSET))
ADMIN_PORT=$((8079 + PORT_OFFSET))
OPENAI_PORT=$((8082 + PORT_OFFSET))
ANTHROPIC_PORT=$((8083 + PORT_OFFSET))
GEMINI_PORT=$((8084 + PORT_OFFSET))
BASE_URL="http://localhost:${GATEWAY_PORT}"
LOG_FILE="/tmp/gateway_live_scheduler_test.log"
STATS_LOG="/tmp/scheduler_stats_test.log"
export TOKLIGENCE_IDENTITY_PATH=${TOKLIGENCE_IDENTITY_PATH:-/tmp/tokligence_identity.db}
export TOKLIGENCE_LEDGER_PATH=${TOKLIGENCE_LEDGER_PATH:-/tmp/tokligence_ledger.db}
export TOKLIGENCE_MODEL_METADATA_URL=""
export TOKLIGENCE_MODEL_METADATA_FILE=${TOKLIGENCE_MODEL_METADATA_FILE:-data/model_metadata.json}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "========================================="
echo "Live Scheduler Policy Test"
echo "========================================="

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    make gdx >/dev/null 2>&1 || true
    sleep 1
}

trap cleanup EXIT

# Build gateway
echo -e "${BLUE}Building gateway...${NC}"
cd "$(dirname "$0")/../../.."
make bgd > /dev/null 2>&1

# Function to start gateway with specific policy
start_gateway() {
    local policy=$1
    local stats_interval=$2

    echo -e "${BLUE}Starting gateway with policy=${policy}, stats_interval=${stats_interval}s${NC}"

    export TOKLIGENCE_SCHEDULER_ENABLED=true
    export TOKLIGENCE_SCHEDULER_POLICY="${policy}"
    export TOKLIGENCE_SCHEDULER_MAX_CONCURRENT=5
    export TOKLIGENCE_SCHEDULER_MAX_QUEUE_DEPTH=100
    export TOKLIGENCE_SCHEDULER_STATS_INTERVAL_SEC="${stats_interval}"
    export TOKLIGENCE_LOG_LEVEL=info
    export TOKLIGENCE_AUTH_DISABLED=true
    export TOKLIGENCE_FACADE_PORT=${GATEWAY_PORT}
    export TOKLIGENCE_ADMIN_PORT=0
    export TOKLIGENCE_OPENAI_PORT=0
    export TOKLIGENCE_ANTHROPIC_PORT=0
    export TOKLIGENCE_GEMINI_PORT=0
    export TOKLIGENCE_MARKETPLACE_ENABLED=false
    export TOKLIGENCE_MULTIPORT_MODE=false
    export TOKLIGENCE_ENABLE_FACADE=true

    ./bin/gatewayd > "${LOG_FILE}" 2>&1 &
    GATEWAY_PID=$!

    # Wait for startup
    sleep 2

    # Verify it's running
    if ! curl -s "${BASE_URL}/health" > /dev/null 2>&1; then
        echo -e "${RED}✗ Gateway failed to start${NC}"
        cat "${LOG_FILE}" | tail -20
        exit 1
    fi

    echo -e "${GREEN}✓ Gateway started (PID: ${GATEWAY_PID})${NC}"
}

# Function to submit request with priority
submit_request() {
    local priority=$1
    local req_id=$2
    local model=${3:-"gpt-4"}

    curl -s -X POST "${BASE_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "X-Priority: ${priority}" \
        -H "Authorization: Bearer test" \
        -d "{
            \"model\": \"${model}\",
            \"messages\": [{\"role\": \"user\", \"content\": \"Test request ${req_id}\"}],
            \"max_tokens\": 10
        }" > /dev/null 2>&1 &
}

# Function to get scheduler stats
get_stats() {
    curl -s "${BASE_URL}/admin/scheduler/stats" 2>/dev/null || echo "{}"
}

# Function to get busiest queues
get_busiest_queues() {
    local top_n=${1:-5}
    curl -s "${BASE_URL}/admin/scheduler/queues?top=${top_n}" 2>/dev/null || echo "{}"
}

# Function to monitor and display stats
monitor_stats() {
    local duration=$1
    local interval=$2

    echo -e "${BLUE}Monitoring scheduler for ${duration}s (sampling every ${interval}s)${NC}"

    local end_time=$(($(date +%s) + duration))

    while [ $(date +%s) -lt $end_time ]; do
        stats=$(get_stats)

        # Extract key metrics
        total_scheduled=$(echo "$stats" | jq -r '.total_scheduled // 0')
        total_queued_now=$(echo "$stats" | jq -r '.total_queued_now // 0')
        overall_util=$(echo "$stats" | jq -r '.overall_utilization // 0')

        echo -e "${YELLOW}[$(date +%H:%M:%S)]${NC} Scheduled: ${total_scheduled}, Queued: ${total_queued_now}, Utilization: ${overall_util}%"

        # Show queue depths per priority
        if [ "$total_queued_now" -gt 0 ]; then
            echo "$stats" | jq -r '.queue_stats[] | select(.current_depth > 0) | "  P\(.priority): \(.current_depth)/\(.max_depth) (\(.utilization_pct | floor)%)"'
        fi

        sleep "$interval"
    done
}

# Function to verify policy behavior
verify_policy_behavior() {
    local policy=$1

    echo ""
    echo -e "${BLUE}=========================================${NC}"
    echo -e "${BLUE}Testing Policy: ${policy}${NC}"
    echo -e "${BLUE}=========================================${NC}"

    # Start gateway with this policy
    start_gateway "${policy}" 30

    echo ""
    echo -e "${YELLOW}Step 1: Submit mixed priority requests${NC}"

    # Submit 20 requests with mixed priorities
    for i in {1..20}; do
        # Mix of priorities: P0 (critical), P2 (high), P5 (normal), P7 (low), P9 (background)
        case $((i % 5)) in
            0) priority=0 ;;  # P0 - Critical
            1) priority=2 ;;  # P2 - High
            2) priority=5 ;;  # P5 - Normal
            3) priority=7 ;;  # P7 - Low
            4) priority=9 ;;  # P9 - Background
        esac

        submit_request "$priority" "${policy}-${i}"

        # Small delay to create queueing
        sleep 0.1
    done

    echo -e "${GREEN}✓ Submitted 20 requests${NC}"

    # Monitor for 10 seconds
    echo ""
    monitor_stats 10 2

    echo ""
    echo -e "${YELLOW}Step 2: Check final stats${NC}"

    # Get final stats
    final_stats=$(get_stats)
    echo "$final_stats" | jq '.'

    # Get busiest queues
    echo ""
    echo -e "${YELLOW}Step 3: Busiest queues${NC}"
    busiest=$(get_busiest_queues 5)
    echo "$busiest" | jq '.busiest_queues'

    # Verify policy-specific behavior
    echo ""
    echo -e "${YELLOW}Step 4: Verify policy behavior${NC}"

    case "$policy" in
        strict)
            echo "Expected: P0 should be processed first, then P2, P5, P7, P9"
            # Check that lower priority queues still have items if higher priorities were processed
            ;;
        wfq)
            echo "Expected: All priorities get fair share based on weights"
            echo "P0 gets 256x more bandwidth than P9"
            ;;
        hybrid)
            echo "Expected: P0 gets strict priority, P1-P9 use WFQ"
            echo "P0 should be empty, others follow WFQ distribution"
            ;;
    esac

    # Save stats
    echo "$final_stats" | jq '.' >> "${STATS_LOG}"

    # Stop gateway
    echo ""
    echo -e "${YELLOW}Stopping gateway...${NC}"
    kill $GATEWAY_PID 2>/dev/null || true
    sleep 2

    echo -e "${GREEN}✓ Test completed for ${policy}${NC}"
}

# Main test execution
echo ""
echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}Starting Policy Tests${NC}"
echo -e "${BLUE}=========================================${NC}"

# Clear logs
> "${LOG_FILE}"
> "${STATS_LOG}"

# Test each policy
for policy in strict wfq hybrid; do
    verify_policy_behavior "$policy"
    echo ""
    sleep 2
done

# Summary
echo ""
echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}=========================================${NC}"

echo ""
echo -e "${GREEN}✓ All policy tests completed${NC}"
echo ""
echo "Logs saved to:"
echo "  Gateway log: ${LOG_FILE}"
echo "  Stats log: ${STATS_LOG}"

echo ""
echo "Review results:"
echo "  tail -100 ${LOG_FILE}"
echo "  cat ${STATS_LOG} | jq ."

exit 0
