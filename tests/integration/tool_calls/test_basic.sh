#!/bin/bash
# Basic tool call test - simple echo command

echo "=== Test 1: Basic Tool Call (echo) ==="
curl -s -N http://localhost:8081/v1/responses \
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
}' 2>&1 | grep -E "^(event:|data:)" | head -20

echo ""
echo "Expected: response.function_call_arguments.done and response.output_item.done events"
echo ""
