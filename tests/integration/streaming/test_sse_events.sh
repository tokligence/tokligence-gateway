#!/bin/bash
# Test to capture and display all SSE events sent by gateway

BASE_URL="${1:-http://localhost:8081}"
API_KEY="${2:-test}"

echo "=== Testing SSE Events ==="
echo "This will show all events sent by the gateway"
echo "Press Ctrl+C to stop"
echo ""

timeout 30 curl -N -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [
      {
        "role": "user",
        "content": "Create a file test.txt with content hello"
      }
    ],
    "tool_choice": "required",
    "tools": [
      {
        "type": "function",
        "name": "apply_patch",
        "description": "Apply a patch to create or modify files",
        "parameters": {
          "type": "object",
          "properties": {
            "command": {
              "type": "string"
            }
          },
          "required": ["command"]
        }
      }
    ],
    "stream": true
  }' 2>&1 | tee /tmp/sse_events.log

echo ""
echo "=== Events saved to /tmp/sse_events.log ==="
