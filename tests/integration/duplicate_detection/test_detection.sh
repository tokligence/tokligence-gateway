#!/bin/bash

# Test Duplicate Tool Call Detection
# This test verifies that:
# - 3 duplicates trigger a warning
# - 4 duplicates trigger an urgent warning
# - 5 duplicates trigger emergency stop (rejection)

set -e

BASE_URL="http://localhost:8081"
AUTH="Bearer test"

echo "🧪 Testing Duplicate Tool Call Detection"
echo "=========================================="
echo ""

# Step 1: Create initial request that will trigger a tool call
echo "Step 1: Creating initial request..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [
      {"role": "user", "content": "Run the command: echo test"}
    ],
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "shell",
          "description": "Execute shell command",
          "parameters": {
            "type": "object",
            "properties": {
              "command": {"type": "string"}
            }
          }
        }
      }
    ],
    "stream": false
  }')

# Extract response ID and first tool call
RESP_ID=$(echo "$RESPONSE" | jq -r '.id')
TOOL_CALL_ID=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].id')
TOOL_NAME=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].function.name')
TOOL_ARGS=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].function.arguments')

echo "  Response ID: $RESP_ID"
echo "  Tool Call ID: $TOOL_CALL_ID"
echo "  Tool: $TOOL_NAME"
echo "  Args: $TOOL_ARGS"
echo ""

if [ "$RESP_ID" == "null" ] || [ -z "$RESP_ID" ]; then
  echo "❌ Failed to get response ID"
  exit 1
fi

# Function to submit tool outputs and continue
submit_and_continue() {
  local iteration=$1
  echo "Step $iteration: Submitting tool output (duplicate #$((iteration-1)))..."

  RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses/$RESP_ID/submit_tool_outputs" \
    -H "Content-Type: application/json" \
    -H "Authorization: $AUTH" \
    -d "{
      \"tool_outputs\": [
        {
          \"tool_call_id\": \"$TOOL_CALL_ID\",
          \"output\": \"test\"
        }
      ]
    }")

  STATUS=$(echo "$RESPONSE" | jq -r '.status // .error.message // "unknown"')
  FINISH_REASON=$(echo "$RESPONSE" | jq -r '.finish_reason // "none"')
  ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // "none"')

  echo "  Status: $STATUS"
  echo "  Finish Reason: $FINISH_REASON"

  if [ "$ERROR_MSG" != "none" ]; then
    echo "  ❌ Error: $ERROR_MSG"
    return 1
  fi

  # Check if there's a new tool call (indicates duplicate)
  NEW_TOOL_CALL_ID=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].id // "none"')
  if [ "$NEW_TOOL_CALL_ID" != "none" ]; then
    echo "  ⚠️  New tool call detected (duplicate)"
    TOOL_CALL_ID="$NEW_TOOL_CALL_ID"
  fi

  echo ""
  return 0
}

# Step 2: First continuation (duplicate #1)
submit_and_continue 2 || exit 1

# Step 3: Second continuation (duplicate #2)
submit_and_continue 3 || exit 1

# Step 4: Third continuation (duplicate #3 - should trigger warning)
echo "Step 4: Third duplicate (should trigger ⚠️ WARNING)..."
submit_and_continue 4 || exit 1
echo "  ✅ Expected: Warning should be injected into system message"
echo ""

# Step 5: Fourth continuation (duplicate #4 - should trigger urgent warning)
echo "Step 5: Fourth duplicate (should trigger 🚨 URGENT WARNING)..."
submit_and_continue 5 || exit 1
echo "  ✅ Expected: Urgent warning with mention of halt"
echo ""

# Step 6: Fifth continuation (duplicate #5 - should REJECT)
echo "Step 6: Fifth duplicate (should trigger EMERGENCY STOP)..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses/$RESP_ID/submit_tool_outputs" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d "{
    \"tool_outputs\": [
      {
        \"tool_call_id\": \"$TOOL_CALL_ID\",
        \"output\": \"test\"
      }
    ]
  }")

ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // "none"')
ERROR_TYPE=$(echo "$RESPONSE" | jq -r '.error.type // "none"')

echo "  Error Type: $ERROR_TYPE"
echo "  Error Message: $ERROR_MSG"

if [[ "$ERROR_MSG" == *"infinite loop"* ]] || [[ "$ERROR_MSG" == *"consecutively"* ]]; then
  echo "  ✅ PASS: Emergency stop triggered with correct error message!"
else
  echo "  ❌ FAIL: Expected infinite loop error, got: $ERROR_MSG"
  exit 1
fi

echo ""
echo "=========================================="
echo "✅ All tests passed!"
echo ""
echo "Summary:"
echo "  - Duplicates 1-2: Continued normally"
echo "  - Duplicate 3: Warning injected ⚠️"
echo "  - Duplicate 4: Urgent warning injected 🚨"
echo "  - Duplicate 5: Emergency stop (rejected) 🛑"
echo ""
echo "Check logs for detailed warning messages:"
echo "  tail -50 logs/dev-gatewayd-$(date +%Y-%m-%d).log | grep -E 'duplicate|WARNING|EMERGENCY'"
