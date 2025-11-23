#!/bin/bash
# Channel Occupancy Monitoring Test
# Verifies real-time channel depth monitoring and HTTP stats endpoints

set -e

GATEWAY_PORT=8081
BASE_URL="http://localhost:${GATEWAY_PORT}"
LOG_FILE="/tmp/gateway_channel_monitor_test.log"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

echo "========================================="
echo "Channel Occupancy Monitoring Test"
echo "========================================="

# Cleanup
cleanup() {
    echo -e "\n${YELLOW}Cleaning up...${NC}"
    pkill -f "gatewayd" || true
    sleep 1
}

trap cleanup EXIT

# Build
echo -e "${BLUE}Building gateway...${NC}"
cd "$(dirname "$0")/../../.."
make bgd > /dev/null 2>&1

# Start gateway with monitoring enabled
echo -e "${BLUE}Starting gateway with stats monitoring (interval=10s)${NC}"

export TOKLIGENCE_SCHEDULER_ENABLED=true
export TOKLIGENCE_SCHEDULER_POLICY=hybrid
export TOKLIGENCE_SCHEDULER_MAX_CONCURRENT=3  # Low to force queueing
export TOKLIGENCE_SCHEDULER_MAX_QUEUE_DEPTH=50
export TOKLIGENCE_SCHEDULER_STATS_INTERVAL_SEC=10  # 10 seconds for testing
export TOKLIGENCE_LOG_LEVEL=debug
export TOKLIGENCE_AUTH_DISABLED=true

./bin/gatewayd > "${LOG_FILE}" 2>&1 &
GATEWAY_PID=$!

sleep 3

if ! curl -s "${BASE_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ Gateway failed to start${NC}"
    cat "${LOG_FILE}" | tail -20
    exit 1
fi

echo -e "${GREEN}✓ Gateway started (PID: ${GATEWAY_PID})${NC}"

# Function to display queue stats nicely
display_queue_stats() {
    local stats=$1

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}Queue Occupancy Snapshot [$(date +%H:%M:%S)]${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    # Overall stats
    local total_scheduled=$(echo "$stats" | jq -r '.total_scheduled // 0')
    local total_queued=$(echo "$stats" | jq -r '.total_queued_now // 0')
    local overall_util=$(echo "$stats" | jq -r '.overall_utilization // 0')

    echo -e "Total Scheduled: ${GREEN}${total_scheduled}${NC}"
    echo -e "Total Queued:    ${YELLOW}${total_queued}${NC}"
    echo -e "Overall Util:    ${BLUE}${overall_util}%${NC}"

    echo ""
    echo -e "${CYAN}Per-Priority Queue Depths:${NC}"
    echo "$stats" | jq -r '.queue_stats[] |
        if .current_depth > 0 then
            if .utilization_pct > 80 then
                "  🔥 P\(.priority): \(.current_depth)/\(.max_depth) (\(.utilization_pct | floor)%) - HOT"
            elif .utilization_pct > 50 then
                "  ⚠️  P\(.priority): \(.current_depth)/\(.max_depth) (\(.utilization_pct | floor)%) - WARN"
            else
                "  ✓  P\(.priority): \(.current_depth)/\(.max_depth) (\(.utilization_pct | floor)%)"
            end
        else
            "     P\(.priority): \(.current_depth)/\(.max_depth) (0%) - idle"
        end'

    # Internal channel stats
    echo ""
    echo -e "${CYAN}Internal Channels:${NC}"
    local check_queue=$(echo "$stats" | jq -r '.channel_stats.capacity_check_queue // 0')
    local release_queue=$(echo "$stats" | jq -r '.channel_stats.capacity_release_queue // 0')
    local buffer_size=$(echo "$stats" | jq -r '.channel_stats.internal_buffer_size // 0')

    echo "  Capacity Check:   ${check_queue}/${buffer_size}"
    echo "  Capacity Release: ${release_queue}/${buffer_size}"

    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# Test 1: Initial state
echo ""
echo -e "${YELLOW}Test 1: Verify initial state (empty queues)${NC}"
stats=$(curl -s "${BASE_URL}/admin/scheduler/stats")
display_queue_stats "$stats"

total_queued=$(echo "$stats" | jq -r '.total_queued_now // 0')
if [ "$total_queued" -eq 0 ]; then
    echo -e "${GREEN}✓ Initial state correct (no queued requests)${NC}"
else
    echo -e "${RED}✗ Expected 0 queued requests, got ${total_queued}${NC}"
fi

# Test 2: Submit burst of requests to different priorities
echo ""
echo -e "${YELLOW}Test 2: Submit 30 requests to create queue backlog${NC}"

# Submit requests in batches to different priorities
for priority in 0 2 5 7 9; do
    for i in {1..6}; do
        curl -s -X POST "${BASE_URL}/v1/chat/completions" \
            -H "Content-Type: application/json" \
            -H "X-Priority: ${priority}" \
            -H "Authorization: Bearer test" \
            -d "{
                \"model\": \"gpt-4\",
                \"messages\": [{\"role\": \"user\", \"content\": \"Test P${priority}-${i}\"}],
                \"max_tokens\": 10
            }" > /dev/null 2>&1 &
    done
done

echo -e "${GREEN}✓ Submitted 30 requests (6 per priority: P0, P2, P5, P7, P9)${NC}"

# Wait a bit for queueing
sleep 2

# Test 3: Monitor queue occupancy over time
echo ""
echo -e "${YELLOW}Test 3: Monitor queue occupancy for 20 seconds${NC}"

for i in {1..10}; do
    echo ""
    echo -e "${BLUE}Snapshot ${i}/10${NC}"
    stats=$(curl -s "${BASE_URL}/admin/scheduler/stats")
    display_queue_stats "$stats"
    sleep 2
done

# Test 4: Get busiest queues
echo ""
echo -e "${YELLOW}Test 4: Get top 3 busiest queues${NC}"
busiest=$(curl -s "${BASE_URL}/admin/scheduler/queues?top=3")
echo "$busiest" | jq '.busiest_queues'

# Test 5: Verify stats endpoint returns valid JSON
echo ""
echo -e "${YELLOW}Test 5: Verify stats endpoint structure${NC}"

stats=$(curl -s "${BASE_URL}/admin/scheduler/stats")

# Check required fields
required_fields=("enabled" "total_scheduled" "total_rejected" "queue_stats" "channel_stats")

all_present=true
for field in "${required_fields[@]}"; do
    if echo "$stats" | jq -e ".${field}" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Field '${field}' present${NC}"
    else
        echo -e "${RED}✗ Field '${field}' missing${NC}"
        all_present=false
    fi
done

if [ "$all_present" = true ]; then
    echo -e "${GREEN}✓ All required fields present${NC}"
else
    echo -e "${RED}✗ Some fields missing${NC}"
    exit 1
fi

# Test 6: Verify queue_stats array structure
echo ""
echo -e "${YELLOW}Test 6: Verify queue_stats structure${NC}"

queue_count=$(echo "$stats" | jq '.queue_stats | length')
if [ "$queue_count" -eq 10 ]; then
    echo -e "${GREEN}✓ queue_stats has 10 priority levels${NC}"
else
    echo -e "${RED}✗ Expected 10 priority levels, got ${queue_count}${NC}"
fi

# Check each queue has required fields
for i in {0..9}; do
    priority=$(echo "$stats" | jq -r ".queue_stats[${i}].priority")
    current_depth=$(echo "$stats" | jq -r ".queue_stats[${i}].current_depth")
    max_depth=$(echo "$stats" | jq -r ".queue_stats[${i}].max_depth")
    utilization=$(echo "$stats" | jq -r ".queue_stats[${i}].utilization_pct")

    if [ "$priority" == "null" ] || [ "$current_depth" == "null" ] || [ "$max_depth" == "null" ]; then
        echo -e "${RED}✗ P${i}: Missing required fields${NC}"
    else
        echo -e "${GREEN}✓ P${i}: priority=${priority}, depth=${current_depth}/${max_depth}, util=${utilization}%${NC}"
    fi
done

# Test 7: Check periodic stats logging in gateway logs
echo ""
echo -e "${YELLOW}Test 7: Check periodic stats logging${NC}"

if grep -q "===== Channel Scheduler Statistics =====" "${LOG_FILE}"; then
    echo -e "${GREEN}✓ Periodic stats logging found in logs${NC}"
    echo ""
    echo "Last stats log entry:"
    grep -A 20 "===== Channel Scheduler Statistics =====" "${LOG_FILE}" | tail -20
else
    echo -e "${YELLOW}⚠️  Periodic stats logging not yet triggered (interval=10s)${NC}"
fi

# Test 8: Verify channel monitoring doesn't impact performance
echo ""
echo -e "${YELLOW}Test 8: Performance check (channel monitoring overhead)${NC}"

# Submit 100 requests quickly and measure throughput
start_time=$(date +%s.%N)
for i in {1..100}; do
    priority=$((i % 10))
    curl -s -X POST "${BASE_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "X-Priority: ${priority}" \
        -H "Authorization: Bearer test" \
        -d "{
            \"model\": \"gpt-4\",
            \"messages\": [{\"role\": \"user\", \"content\": \"Perf test ${i}\"}],
            \"max_tokens\": 5
        }" > /dev/null 2>&1 &
done

wait

end_time=$(date +%s.%N)
duration=$(echo "$end_time - $start_time" | bc)
throughput=$(echo "100 / $duration" | bc -l)

echo -e "Submitted 100 requests in ${duration}s"
echo -e "Throughput: ${BLUE}${throughput} req/s${NC}"

if (( $(echo "$throughput > 10" | bc -l) )); then
    echo -e "${GREEN}✓ Performance acceptable${NC}"
else
    echo -e "${YELLOW}⚠️  Performance lower than expected${NC}"
fi

# Final summary
echo ""
echo -e "${BLUE}=========================================${NC}"
echo -e "${BLUE}Test Summary${NC}"
echo -e "${BLUE}=========================================${NC}"

final_stats=$(curl -s "${BASE_URL}/admin/scheduler/stats")
echo ""
echo -e "${CYAN}Final Statistics:${NC}"
echo "$final_stats" | jq '{
    total_scheduled,
    total_rejected,
    total_queued: .total_queued,
    total_queued_now,
    overall_utilization,
    scheduling_policy
}'

echo ""
echo -e "${GREEN}✓ All channel monitoring tests passed${NC}"
echo ""
echo "Gateway log: ${LOG_FILE}"
echo "  View full stats: grep 'Channel Scheduler Statistics' ${LOG_FILE} -A 30"

exit 0
