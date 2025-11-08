#!/bin/bash
# Diagnose tool call format by comparing key fields

echo "=== Diagnostic: Tool Call Response Format ==="
echo ""

# Capture a tool call response
curl -s http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "Use the shell tool to write hello to test.txt"}]}
  ],
  "tools": [{
    "type": "function",
    "name": "shell",
    "description": "Execute a shell command",
    "parameters": {
      "type": "object",
      "properties": {
        "command": {"type": "string", "description": "The shell command to execute"}
      },
      "required": ["command"]
    }
  }],
  "tool_choice": true,
  "stream": false
}' | jq . > /tmp/our_tool_response.json

echo "Non-streaming response structure:"
cat /tmp/our_tool_response.json | jq '{
  id,
  object,
  model,
  output_type: .output[0].type,
  output_structure: .output[0] | keys
}'

echo ""
echo "Tool call details:"
cat /tmp/our_tool_response.json | jq '.output[0].content[0]'

echo ""
echo "Checking for required fields..."
echo -n "- output[0].type: "
cat /tmp/our_tool_response.json | jq -r '.output[0].type // "MISSING"'

echo -n "- output[0].content[0].type: "
cat /tmp/our_tool_response.json | jq -r '.output[0].content[0].type // "MISSING"'

echo -n "- output[0].content[0].name: "
cat /tmp/our_tool_response.json | jq -r '.output[0].content[0].name // "MISSING"'

echo -n "- output[0].content[0].call_id: "
cat /tmp/our_tool_response.json | jq -r '.output[0].content[0].call_id // "MISSING"'

echo -n "- output[0].content[0].arguments: "
cat /tmp/our_tool_response.json | jq -r '.output[0].content[0].arguments // "MISSING"' | head -c 50
echo "..."

echo ""
echo "Full response saved to: /tmp/our_tool_response.json"
