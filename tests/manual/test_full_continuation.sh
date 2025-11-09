#!/bin/bash
set -e

echo "=== Test: Complete tool call continuation flow ==="
echo ""

echo "Step 1: Initial request with tool (should trigger tool call and close stream)"
RESPONSE1=$(timeout 15 curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "input": [
      {"role": "user", "content": [{"type": "input_text", "text": "Use the shell tool to run: echo hello"}]}
    ],
    "tools": [{
      "type": "function",
      "name": "shell",
      "description": "Run shell command",
      "parameters": {
        "type": "object",
        "properties": {
          "command": {"type": "array", "items": {"type": "string"}}
        },
        "required": ["command"]
      }
    }],
    "tool_choice": true,
    "stream": true
  }' 2>&1)

echo "$RESPONSE1" | head -30
echo ""

# Extract call_id
CALL_ID=$(echo "$RESPONSE1" | grep -oP '"call_id":"call_[0-9]+"' | head -1 | grep -oP 'call_[0-9]+')
echo "Extracted call_id: $CALL_ID"

if [ -z "$CALL_ID" ]; then
  echo "ERROR: No tool call found"
  exit 1
fi

# Check that stream ended with [DONE]
if echo "$RESPONSE1" | grep -q "data: \[DONE\]"; then
  echo "✓ Stream closed correctly with [DONE]"
else
  echo "✗ Stream did not close with [DONE]"
  exit 1
fi

echo ""
echo "Step 2: Send continuation request with function_call_output"
RESPONSE2=$(timeout 15 curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d "{
    \"model\": \"claude-3-5-sonnet-20241022\",
    \"input\": [
      {
        \"type\": \"function_call_output\",
        \"call_id\": \"$CALL_ID\",
        \"output\": \"hello\"
      }
    ],
    \"stream\": true
  }" 2>&1)

echo "$RESPONSE2" | head -40
echo ""

# Check for successful completion
if echo "$RESPONSE2" | grep -q "response.completed"; then
  echo "✓ Continuation completed successfully"
else
  echo "✗ Continuation did not complete"
  exit 1
fi

# Check for error
if echo "$RESPONSE2" | grep -q "event: error"; then
  echo "✗ ERROR in continuation:"
  echo "$RESPONSE2" | grep -A 2 "event: error"
  exit 1
fi

echo ""
echo "=== ✓ ALL TESTS PASSED ==="
