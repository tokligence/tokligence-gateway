#!/bin/bash
# Test with Codex's actual shell tool schema (command as string, not array)

echo "=== Testing Codex Shell Tool Format (string command) ==="

curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "Use the shell tool to run: echo test123 > output.txt"}]}
  ],
  "tools": [{
    "type": "function",
    "name": "shell",
    "description": "Execute a shell command",
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
  }],
  "tool_choice": true,
  "stream": true
}' 2>&1 | tee /tmp/codex_shell_test.txt

echo ""
echo "Checking arguments format..."
grep -A1 'response.function_call_arguments.done' /tmp/codex_shell_test.txt | grep '"arguments"' | head -1

echo ""
echo "Expected: command should be a STRING, not an array"
echo 'e.g.: {"command": "echo test123 > output.txt"}'
echo 'NOT:  {"command": ["echo", "test123", ">", "output.txt"]}'
