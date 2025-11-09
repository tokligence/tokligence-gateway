#!/bin/bash
# Non-streaming tool call test

echo "=== Test 4: Non-Streaming Tool Call ==="
curl -s http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "Use the shell tool to run: pwd"}]}
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
  "stream": false
}' | jq '.output[0].content[0]'

echo ""
echo "Expected: JSON with type=function_call, name=shell, arguments containing pwd"
echo ""
