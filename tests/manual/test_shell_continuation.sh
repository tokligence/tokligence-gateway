#!/bin/bash

echo "=== Step 1: Initial request with tool call ==="
RESPONSE=$(timeout 15 curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Execute: echo hello"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "shell",
        "description": "Execute shell command",
        "parameters": {
          "type": "object",
          "properties": {
            "command": {"type": "string", "description": "Shell command to execute"}
          },
          "required": ["command"]
        }
      }
    }],
    "tool_choice": "required",
    "stream": true
  }')

echo "$RESPONSE" | head -80
echo ""
echo "=== Extracting response ID and call ID ==="

RESP_ID=$(echo "$RESPONSE" | grep -oP '"id":"resp_[0-9]+"' | head -1 | grep -oP 'resp_[0-9]+')
CALL_ID=$(echo "$RESPONSE" | grep -oP '"call_id":"call_[0-9]+"' | head -1 | grep -oP 'call_[0-9]+')

echo "Response ID: $RESP_ID"
echo "Call ID: $CALL_ID"

if [ -z "$CALL_ID" ]; then
  echo "ERROR: No tool call found in response"
  exit 1
fi

echo ""
echo "=== Step 2: Submit tool output via new request with function_call_output ==="

RESPONSE2=$(timeout 15 curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d "{
    \"model\": \"claude-3-5-haiku-latest\",
    \"input\": [
      {
        \"type\": \"function_call_output\",
        \"call_id\": \"$CALL_ID\",
        \"output\": \"hello\"
      }
    ],
    \"stream\": true
  }")

echo "$RESPONSE2" | head -50
echo ""
echo "=== Checking for continuation success ==="

if echo "$RESPONSE2" | grep -q "response.completed"; then
  echo "✓ SUCCESS: Continuation completed"
else
  echo "✗ FAIL: Continuation did not complete"
fi
