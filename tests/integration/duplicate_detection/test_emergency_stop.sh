#!/usr/bin/env bash
set -euo pipefail

# Test: Duplicate Tool Call Detection and Emergency Stop
# This test verifies that submitting the same tool output 5 times triggers an emergency stop

# Get script directory and project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

BASE_URL=${BASE_URL:-http://localhost:8081}
REQ_JSON="$PROJECT_ROOT/tests/fixtures/tool_calls/basic_request.json"
AUTH_HEADER=()

echo "🧪 Testing Duplicate Tool Call Emergency Stop"
echo "=============================================="
echo ""

# Step 1: Create initial request that triggers a tool call
echo "Step 1: Creating initial request with tool call..."
LOG=$(mktemp)
cleanup() { rm -f "$LOG"; }
trap cleanup EXIT

curl -sS -N "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  "${AUTH_HEADER[@]}" \
  -d @"$REQ_JSON" >"$LOG" &
STREAM_PID=$!

# Wait for required_action
for ((i=0;i<60;i++)); do
  if grep -q 'required_action' "$LOG" 2>/dev/null; then
    break
  fi
  if ! kill -0 "$STREAM_PID" 2>/dev/null; then
    echo "❌ Stream ended before required_action"
    cat "$LOG"
    exit 1
  fi
  sleep 1
done

# Parse response ID and tool call ID
PARSE=$(python3 - "$LOG" <<'PY'
import json, sys
log = sys.argv[1]
resp_id = None
call_id = None
for line in open(log):
    line=line.strip()
    if not line.startswith('data: '):
        continue
    payload = line[6:]
    if payload == '[DONE]':
        continue
    try:
        event = json.loads(payload)
    except:
        continue
    if event.get('type') == 'response.created':
        resp_id = event.get('response', {}).get('id')
    if event.get('type') == 'response.completed':
        resp = event.get('response', {})
        required = resp.get('required_action') or {}
        sto = required.get('submit_tool_outputs', {})
        calls = sto.get('tool_calls') or []
        if calls:
            call_id = calls[0].get('id')
            break
if not resp_id or not call_id:
    print('PARSE_ERROR')
    sys.exit(1)
print(resp_id)
print(call_id)
PY
) || { echo "❌ Failed to parse response"; cat "$LOG"; exit 1; }

RESP_ID=$(echo "$PARSE" | sed -n '1p')
CALL_ID=$(echo "$PARSE" | sed -n '2p')

echo "  Response ID: $RESP_ID"
echo "  Tool Call ID: $CALL_ID"
echo ""

# Function to submit tool output
submit_tool_output() {
  local iteration=$1
  local expect_error=${2:-false}

  echo "Step $iteration: Submitting tool output..."

  PAYLOAD=$(jq -n --arg id "$CALL_ID" --arg output '{"status":"ok"}' \
    '{tool_outputs:[{tool_call_id:$id, output:$output}]}')

  RESP=$(curl -sS -X POST "$BASE_URL/v1/responses/$RESP_ID/submit_tool_outputs" \
    -H "Content-Type: application/json" \
    "${AUTH_HEADER[@]}" \
    -d "$PAYLOAD")

  if [ "$expect_error" == "true" ]; then
    # Check for error indicating infinite loop
    ERROR_MSG=$(echo "$RESP" | jq -r '.error.message // "none"')
    if [[ "$ERROR_MSG" == *"infinite loop"* ]] || [[ "$ERROR_MSG" == *"consecutively"* ]]; then
      echo "  ✅ Emergency stop triggered: $ERROR_MSG"
      return 0
    else
      echo "  ❌ Expected emergency stop error, got: $RESP"
      return 1
    fi
  else
    # Check for accepted or completion
    if echo "$RESP" | grep -q 'accepted\|completed\|incomplete'; then
      echo "  ✅ Tool output accepted"
      return 0
    else
      echo "  ⚠️  Response: $RESP"
      return 1
    fi
  fi
}

# Step 2-6: Submit same tool output 5 times
submit_tool_output 2 false || exit 1
sleep 1

submit_tool_output 3 false || exit 1
sleep 1

# After 3 duplicates, should get warning (but still accepted)
echo "  ⚠️  After 3 duplicates: Warning should be injected"
submit_tool_output 4 false || exit 1
sleep 1

# After 4 duplicates, should get urgent warning (but still accepted)
echo "  🚨 After 4 duplicates: Urgent warning should be injected"
submit_tool_output 5 false || exit 1
sleep 1

# After 5 duplicates, should REJECT with emergency stop
echo "  🛑 After 5 duplicates: Should trigger EMERGENCY STOP..."
submit_tool_output 6 true || exit 1

echo ""
echo "=============================================="
echo "✅ All duplicate detection tests passed!"
echo ""
echo "Summary:"
echo "  - Duplicates 1-2: Accepted ✅"
echo "  - Duplicate 3: Accepted with warning ⚠️"
echo "  - Duplicate 4: Accepted with urgent warning 🚨"
echo "  - Duplicate 5: REJECTED with emergency stop 🛑"
echo ""
echo "Check logs for warning messages:"
echo "  tail -100 logs/dev-gatewayd-*.log | grep -E 'duplicate|WARNING|EMERGENCY'"
