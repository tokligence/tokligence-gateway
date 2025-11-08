#!/bin/bash
# Debug non-streaming response

echo "=== Non-Streaming Tool Call (Full Response) ==="
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
}'

echo ""
