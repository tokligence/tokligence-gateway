#!/bin/bash
# Capture SSE events to compare with OpenAI format

BASE_URL="${1:-http://localhost:8081}"

echo "=== Capturing SSE Events for Codex Comparison ==="
echo ""
echo "Sending request with tool call..."
echo ""

timeout 5 curl -N -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Create test.txt"}],
    "tools": [{
      "type": "function",
      "name": "shell",
      "description": "Run shell command",
      "parameters": {
        "type": "object",
        "properties": {"command": {"type": "string"}},
        "required": ["command"]
      }
    }],
    "tool_choice": "required",
    "stream": true
  }' 2>&1 | tee /tmp/codex_sse_capture.txt

echo ""
echo "=== Events captured to /tmp/codex_sse_capture.txt ==="
echo ""
echo "Key events to check:"
grep "event:" /tmp/codex_sse_capture.txt | sort | uniq -c
echo ""
echo "=== Checking for required_action ==="
grep -A5 "response.required_action" /tmp/codex_sse_capture.txt | head -20
