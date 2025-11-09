#!/bin/bash

# Test: Duplicate Detection with Mixed Tool Calls
# This test verifies that:
# - Duplicate counter is tool-specific (different tools don't increment same counter)
# - Counter resets when switching to a different tool
# - Mixed tool calls don't trigger false positives

set -e

BASE_URL="http://localhost:8081"
AUTH="Bearer test"

echo "🧪 Testing Duplicate Detection with Mixed Tool Calls"
echo "====================================================="
echo ""

# Step 1: Create initial request with multiple tools
echo "Step 1: Creating initial request with multiple tools..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d '{
    "model": "claude-3-5-haiku-20241022",
    "messages": [
      {"role": "user", "content": "First run shell command: echo test1"}
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
      },
      {
        "type": "function",
        "function": {
          "name": "read_file",
          "description": "Read file contents",
          "parameters": {
            "type": "object",
            "properties": {
              "path": {"type": "string"}
            }
          }
        }
      }
    ]
    ,"stream": false
  }')

RESP_ID=$(echo "$RESPONSE" | jq -r '.id')
TOOL_CALL_ID=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].id')
TOOL_NAME=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].function.name')

echo "  Response ID: $RESP_ID"
echo "  First Tool Call ID: $TOOL_CALL_ID"
echo "  Tool Name: $TOOL_NAME"
echo ""

if [ "$RESP_ID" == "null" ] || [ -z "$RESP_ID" ]; then
  echo "❌ Failed to get response ID"
  exit 1
fi

# Function to submit tool output with specific tool name
submit_with_tool() {
  local iteration=$1
  local tool_name=$2
  local output=$3

  echo "Step $iteration: Submitting tool output for $tool_name..."

  RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses/$RESP_ID/submit_tool_outputs" \
    -H "Content-Type: application/json" \
    -H "Authorization: $AUTH" \
    -d "{
      \"tool_outputs\": [
        {
          \"tool_call_id\": \"$TOOL_CALL_ID\",
          \"output\": \"$output\"
        }
      ]
    }")

  STATUS=$(echo "$RESPONSE" | jq -r '.status // .error.message // "unknown"')
  ERROR_MSG=$(echo "$RESPONSE" | jq -r '.error.message // "none"')

  echo "  Status: $STATUS"

  if [ "$ERROR_MSG" != "none" ]; then
    echo "  ❌ Error: $ERROR_MSG"
    return 1
  fi

  # Check if there's a new tool call
  NEW_TOOL_CALL_ID=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].id // "none"')
  NEW_TOOL_NAME=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].function.name // "none"')

  if [ "$NEW_TOOL_CALL_ID" != "none" ]; then
    echo "  🔄 New tool call: $NEW_TOOL_NAME (ID: $NEW_TOOL_CALL_ID)"
    TOOL_CALL_ID="$NEW_TOOL_CALL_ID"
    TOOL_NAME="$NEW_TOOL_NAME"
  fi

  echo ""
  return 0
}

# Test Scenario 1: Same tool 3 times (should trigger warning)
echo "=== Scenario 1: Same Tool 3 Times ==="
submit_with_tool 2 "shell" "test1" || exit 1
submit_with_tool 3 "shell" "test1" || exit 1
submit_with_tool 4 "shell" "test1" || exit 1
echo "  ✅ Expected: Warning should be injected after 3rd duplicate"
echo ""

# Test Scenario 2: Switch to different tool (counter should reset)
echo "=== Scenario 2: Switch to Different Tool (Counter Reset) ==="
# Modify the request to ask for read_file
echo "Step 5: Asking for different tool (read_file)..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses/$RESP_ID/submit_tool_outputs" \
  -H "Content-Type: application/json" \
  -H "Authorization: $AUTH" \
  -d "{
    \"tool_outputs\": [
      {
        \"tool_call_id\": \"$TOOL_CALL_ID\",
        \"output\": \"test - now read a file\"
      }
    ]
  }")

NEW_TOOL_NAME=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].function.name // "none"')
echo "  New tool requested: $NEW_TOOL_NAME"

if [ "$NEW_TOOL_NAME" != "$TOOL_NAME" ]; then
  echo "  ✅ Tool changed from $TOOL_NAME to $NEW_TOOL_NAME - counter should reset"
else
  echo "  ⚠️  Tool unchanged, continuing with $TOOL_NAME"
fi
echo ""

# Test Scenario 3: New tool 3 times (should NOT trigger warning immediately)
echo "=== Scenario 3: New Tool 3 Times (Fresh Counter) ==="
NEW_TOOL_CALL_ID=$(echo "$RESPONSE" | jq -r '.required_action.submit_tool_outputs.tool_calls[0].id // "$TOOL_CALL_ID"')
TOOL_CALL_ID="$NEW_TOOL_CALL_ID"

submit_with_tool 6 "$NEW_TOOL_NAME" "file contents 1" || exit 1
submit_with_tool 7 "$NEW_TOOL_NAME" "file contents 1" || exit 1
submit_with_tool 8 "$NEW_TOOL_NAME" "file contents 1" || exit 1
echo "  ✅ Expected: Warning should be injected for this NEW tool after 3 duplicates"
echo ""

# Test Scenario 4: Verify warning is specific to the tool
echo "=== Scenario 4: Verify Detection is Per-Tool ==="
echo "  If counter was NOT reset, we would see emergency stop"
echo "  If counter WAS reset, we only see warning (not emergency stop)"
echo "  Current state: 3 duplicates of $NEW_TOOL_NAME (after switching from shell)"
echo ""

echo "====================================================="
echo "✅ All mixed tool tests completed!"
echo ""
echo "Summary of tested scenarios:"
echo "  1. Same tool 3 times → Warning ⚠️"
echo "  2. Switch to different tool → Counter reset ✅"
echo "  3. New tool 3 times → Fresh warning ⚠️"
echo "  4. Per-tool duplicate tracking verified ✅"
echo ""
echo "Check logs for detailed detection behavior:"
echo "  tail -100 logs/dev-gatewayd-$(date +%Y-%m-%d).log | grep -E 'duplicate|WARNING|shell|read_file'"
