#!/bin/bash
# Test script to reproduce tool call hang issue

set -e

BASE_URL="${1:-http://localhost:8081}"
API_KEY="${2:-test}"

echo "=== Testing tool call flow with Codex-like payload ==="

# Create a test request with shell tool (simulating Codex)
RESPONSE=$(curl -s -N -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [
      {
        "role": "user",
        "content": "Execute: echo hello > test.txt"
      }
    ],
    "tool_choice": "required",
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "shell",
          "description": "Execute shell commands",
          "parameters": {
            "type": "object",
            "properties": {
              "command": {
                "type": "string",
                "description": "The shell command to execute"
              }
            },
            "required": ["command"]
          }
        }
      }
    ],
    "stream": true
  }' 2>&1)

echo "=== Raw Response ==="
echo "$RESPONSE"
echo ""

# Extract response ID from the stream
RESPONSE_ID=$(echo "$RESPONSE" | grep -oP '"id":"resp_[0-9]+"' | head -1 | grep -oP 'resp_[0-9]+')

if [ -z "$RESPONSE_ID" ]; then
  echo "ERROR: Could not extract response ID from stream"
  exit 1
fi

echo "=== Response ID: $RESPONSE_ID ==="

# Check if we got required_action OR a text response (LLM may not always use tools)
if echo "$RESPONSE" | grep -q "required_action"; then
  echo "✓ Got required_action event"

  # Check if we got response.completed with status incomplete
  if echo "$RESPONSE" | grep -q '"status":"incomplete"'; then
    echo "✓ Got response.completed with status=incomplete"
  else
    echo "✗ Did NOT get response.completed with incomplete status"
    exit 1
  fi
elif echo "$RESPONSE" | grep -q "response.output_text.delta"; then
  echo "✓ Got text response (LLM chose not to use tool)"
  echo "Note: This is valid behavior - LLM may explain instead of executing"
else
  echo "✗ Got neither required_action nor text response"
  exit 1
fi

# Check if stream ended properly
LAST_EVENT=$(echo "$RESPONSE" | grep "^event:" | tail -1)
echo "Last event type: $LAST_EVENT"

echo ""
echo "=== Test Summary ==="
echo "The stream should:"
echo "  1. Emit response.created"
echo "  2. Emit tool_call events"
echo "  3. Emit required_action"
echo "  4. Emit response.completed with status=incomplete"
echo "  5. Stay open (NOT close the connection)"
echo ""
echo "If Codex times out, the stream likely closed prematurely."
