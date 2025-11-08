#!/bin/bash
# Tool call test with math calculation

echo "=== Test 2: Math Tool Call ==="
curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "Use the calculator to compute 25 * 4"}]}
  ],
  "tools": [{
    "type": "function",
    "name": "calculator",
    "description": "Perform mathematical calculations",
    "parameters": {
      "type": "object",
      "properties": {
        "operation": {"type": "string", "enum": ["add", "subtract", "multiply", "divide"]},
        "a": {"type": "number"},
        "b": {"type": "number"}
      },
      "required": ["operation", "a", "b"]
    }
  }],
  "tool_choice": true,
  "stream": true
}' 2>&1 | grep -E "^(event:|data:)" | head -20

echo ""
echo "Expected: tool call with operation=multiply, a=25, b=4"
echo ""
