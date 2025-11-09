#!/bin/bash

echo "=== Step 1: Initial request (tool call expected) ==="
RESPONSE=$(timeout 15 curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Run this command: echo test123"}],
    "tools": [{
      "type": "function",
      "function": {
        "name": "shell",
        "description": "Execute shell command",
        "parameters": {
          "type": "object",
          "properties": {
            "command": {"type": "string"}
          },
          "required": ["command"]
        }
      }
    }],
    "stream": true
  }')

echo "$RESPONSE" | grep "event:" | head -20
echo ""

CALL_ID=$(echo "$RESPONSE" | grep -oP '"call_id":"call_[0-9]+"' | head -1 | grep -oP 'call_[0-9]+')
echo "Call ID: $CALL_ID"

if [ -n "$CALL_ID" ]; then
  echo "✓ Tool call detected"
  echo ""
  echo "=== Step 2: Send function_call_output ==="
  
  RESPONSE2=$(timeout 15 curl -s -N http://localhost:8081/v1/responses \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test" \
    -d "{
      \"model\": \"claude-3-5-haiku-latest\",
      \"input\": [
        {
          \"type\": \"function_call_output\",
          \"call_id\": \"$CALL_ID\",
          \"output\": \"test123\"
        }
      ],
      \"stream\": true
    }")
  
  echo "$RESPONSE2" | head -30
  
  if echo "$RESPONSE2" | grep -q "response.completed"; then
    echo "✓ Continuation successful"
  fi
else
  echo "✗ No tool call found"
fi
