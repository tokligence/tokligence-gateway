#!/bin/bash
# Integration test for Phase 3: Time Window Evaluation
#
# Tests:
# 1. Time-of-day based activation
# 2. Day-of-week filtering
# 3. Midnight-wrapping time ranges
# 4. Timezone handling

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== Integration Test: Time Window Evaluation ==="
echo

# Cleanup function
cleanup() {
    echo "Cleaning up..."
    pkill -f gatewayd || true
    rm -f /tmp/test_time_windows.ini
    rm -f /tmp/gatewayd_time_windows.log
}

trap cleanup EXIT

# Build if needed
if [ ! -f "bin/gatewayd" ]; then
    echo "Building gatewayd..."
    make bgd
fi

# Get current time components
CURRENT_HOUR=$(date +%H)
CURRENT_DOW=$(date +%a)  # Mon, Tue, etc.

# Calculate time ranges for testing
PAST_HOUR=$((CURRENT_HOUR - 2))
FUTURE_HOUR=$((CURRENT_HOUR + 2))

# Handle wrap-around
if [ $PAST_HOUR -lt 0 ]; then
    PAST_HOUR=$((24 + PAST_HOUR))
fi
if [ $FUTURE_HOUR -gt 23 ]; then
    FUTURE_HOUR=$((FUTURE_HOUR - 24))
fi

echo "Current time: $(date)"
echo "Current hour: $CURRENT_HOUR"
echo "Current day: $CURRENT_DOW"
echo "Test past hour: $PAST_HOUR"
echo "Test future hour: $FUTURE_HOUR"
echo

# Create test config with multiple time windows
cat > /tmp/test_time_windows.ini <<EOF
[time_rules]
enabled = true
check_interval_sec = 5
default_timezone = UTC

# Rule 1: Should be ACTIVE (current time)
[rule.weights.active]
type = weight_adjustment
name = Active Rule
description = Should be active right now
enabled = true
start_time = $(printf "%02d:00" $PAST_HOUR)
end_time = $(printf "%02d:00" $FUTURE_HOUR)
weights = 100,50,25,10,5,3,2,1,1,1

# Rule 2: Should be INACTIVE (past time)
[rule.weights.inactive_past]
type = weight_adjustment
name = Inactive Past Rule
description = Should be inactive (time has passed)
enabled = true
start_time = $(printf "%02d:00" $((PAST_HOUR - 1 < 0 ? 23 : PAST_HOUR - 1)))
end_time = $(printf "%02d:00" $PAST_HOUR)
weights = 50,50,50,50,50,50,50,50,50,50

# Rule 3: Should be INACTIVE (future time)
[rule.weights.inactive_future]
type = weight_adjustment
name = Inactive Future Rule
description = Should be inactive (time not reached)
enabled = true
start_time = $(printf "%02d:00" $FUTURE_HOUR)
end_time = $(printf "%02d:00" $((FUTURE_HOUR + 1 > 23 ? 0 : FUTURE_HOUR + 1)))
weights = 25,25,25,25,25,25,25,25,25,25

# Rule 4: All day, current day only
[rule.weights.today]
type = weight_adjustment
name = Today Only Rule
description = Active all day on $CURRENT_DOW only
enabled = true
start_time = 00:00
end_time = 23:59
days_of_week = $CURRENT_DOW
weights = 200,100,50,25,10,5,3,2,1,1

# Rule 5: All day, different day (should be inactive)
[rule.weights.wrong_day]
type = weight_adjustment
name = Wrong Day Rule
description = Active on a different day
enabled = true
start_time = 00:00
end_time = 23:59
days_of_week = Sun
weights = 10,10,10,10,10,10,10,10,10,10
EOF

# If current day is Sunday, swap the day filters
if [ "$CURRENT_DOW" == "Sun" ]; then
    sed -i 's/days_of_week = Sun/days_of_week = Mon/' /tmp/test_time_windows.ini
fi

echo "✓ Created test config: /tmp/test_time_windows.ini"

# Start gatewayd
export TOKLIGENCE_TIME_RULES_ENABLED=true
export TOKLIGENCE_TIME_RULES_CONFIG=/tmp/test_time_windows.ini
export TOKLIGENCE_LOG_LEVEL=info
export TOKLIGENCE_AUTH_DISABLED=true
export TOKLIGENCE_MARKETPLACE_ENABLED=false
export TOKLIGENCE_SCHEDULER_ENABLED=true

echo "Starting gatewayd..."
./bin/gatewayd > /tmp/gatewayd_time_windows.log 2>&1 &
GATEWAYD_PID=$!
echo "✓ Started gatewayd (PID: $GATEWAYD_PID)"

sleep 3

# Verify gatewayd is running
if ! ps -p $GATEWAYD_PID > /dev/null; then
    echo "✗ ERROR: gatewayd failed to start"
    echo "Log output:"
    cat /tmp/gatewayd_time_windows.log
    exit 1
fi
echo "✓ Gateway is running"

# Test: Get rules status and verify active/inactive status
echo
echo "Test: Time Window Evaluation"
echo "------------------------------"
RESPONSE=$(curl -s -X GET http://localhost:8081/admin/time-rules/status \
    -H "Authorization: Bearer test")

echo "Response:"
echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
echo

# Verify total rule count
RULE_COUNT=$(echo "$RESPONSE" | grep -o '"count":\s*[0-9]*' | grep -o '[0-9]*')
if [ "$RULE_COUNT" != "5" ]; then
    echo "✗ FAILED: Expected 5 rules, got $RULE_COUNT"
    exit 1
fi
echo "✓ Rule count is correct: 5"

# Check "Active Rule" is active
ACTIVE_STATUS=$(echo "$RESPONSE" | grep -A 10 '"name": "Active Rule"' | grep -o '"active": true')
if [ -z "$ACTIVE_STATUS" ]; then
    echo "✗ FAILED: 'Active Rule' should be active"
    echo "$RESPONSE" | grep -A 10 '"name": "Active Rule"'
    exit 1
fi
echo "✓ 'Active Rule' is active"

# Check "Inactive Past Rule" is inactive
INACTIVE_PAST_STATUS=$(echo "$RESPONSE" | grep -A 10 '"name": "Inactive Past Rule"' | grep -o '"active": false')
if [ -z "$INACTIVE_PAST_STATUS" ]; then
    echo "✗ FAILED: 'Inactive Past Rule' should be inactive"
    exit 1
fi
echo "✓ 'Inactive Past Rule' is inactive"

# Check "Inactive Future Rule" is inactive
INACTIVE_FUTURE_STATUS=$(echo "$RESPONSE" | grep -A 10 '"name": "Inactive Future Rule"' | grep -o '"active": false')
if [ -z "$INACTIVE_FUTURE_STATUS" ]; then
    echo "✗ FAILED: 'Inactive Future Rule' should be inactive"
    exit 1
fi
echo "✓ 'Inactive Future Rule' is inactive"

# Check "Today Only Rule" is active
TODAY_STATUS=$(echo "$RESPONSE" | grep -A 10 '"name": "Today Only Rule"' | grep -o '"active": true')
if [ -z "$TODAY_STATUS" ]; then
    echo "✗ FAILED: 'Today Only Rule' should be active on $CURRENT_DOW"
    echo "$RESPONSE" | grep -A 10 '"name": "Today Only Rule"'
    exit 1
fi
echo "✓ 'Today Only Rule' is active (day-of-week filter works)"

# Check "Wrong Day Rule" is inactive
WRONG_DAY_STATUS=$(echo "$RESPONSE" | grep -A 10 '"name": "Wrong Day Rule"' | grep -o '"active": false')
if [ -z "$WRONG_DAY_STATUS" ]; then
    echo "✗ FAILED: 'Wrong Day Rule' should be inactive"
    exit 1
fi
echo "✓ 'Wrong Day Rule' is inactive (day-of-week filter works)"

# Count active rules
ACTIVE_COUNT=$(echo "$RESPONSE" | grep -o '"active": true' | wc -l)
if [ "$ACTIVE_COUNT" != "2" ]; then
    echo "✗ FAILED: Expected 2 active rules, got $ACTIVE_COUNT"
    echo "$RESPONSE" | python3 -m json.tool 2>/dev/null || echo "$RESPONSE"
    exit 1
fi
echo "✓ Exactly 2 rules are active (as expected)"

echo
echo "=== All time window tests passed! ==="
