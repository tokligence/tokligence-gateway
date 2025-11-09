#!/bin/bash

echo "=== Step 1: Initial request with tool call ==="
RESPONSE=$(curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "What is 2+2? Use the calculator tool."}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "calculator",
        "description": "Perform calculation",
        "parameters": {
          "type": "object",
          "properties": {
            "expression": {"type": "string"}
          },
          "required": ["expression"]
        }
      }
    }],
    "stream": true
  }')

echo "$RESPONSE"
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

RESPONSE2=$(curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d "{
    \"model\": \"claude-3-5-haiku-latest\",
    \"input\": [
      {
        \"type\": \"function_call_output\",
        \"call_id\": \"$CALL_ID\",
        \"output\": \"4\"
      }
    ],
    \"stream\": true
  }")

echo "$RESPONSE2"
echo ""
echo "=== Test Complete ==="
