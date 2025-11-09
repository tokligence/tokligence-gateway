#!/bin/bash
# Verify the output format in response.completed

echo "=== Verifying output format in response.completed ==="

curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [{"role": "user", "content": [{"type": "input_text", "text": "Use shell to write hello to test.txt"}]}],
  "tools": [{
    "type": "function",
    "name": "shell",
    "description": "Execute command",
    "parameters": {
      "type": "object",
      "properties": {"command": {"type": "string"}},
      "required": ["command"]
    }
  }],
  "tool_choice": true,
  "stream": true
}' 2>&1 | tee /tmp/verify_output.txt | grep 'response.completed' -A1 | tail -1 > /tmp/completed_event.json

echo ""
echo "Output structure:"
cat /tmp/completed_event.json | jq '.response.output[0]'

echo ""
echo "Checking format..."
OUTPUT_TYPE=$(cat /tmp/completed_event.json | jq -r '.response.output[0].type')
HAS_ROLE=$(cat /tmp/completed_event.json | jq '.response.output[0].role')
HAS_CONTENT=$(cat /tmp/completed_event.json | jq '.response.output[0].content')

echo "- output[0].type: $OUTPUT_TYPE"
echo "- output[0].role: $HAS_ROLE"
echo "- output[0].content exists: $([ "$HAS_CONTENT" != "null" ] && echo "YES" || echo "NO")"

if [ "$OUTPUT_TYPE" == "message" ]; then
    echo "✓ Correct: output type is 'message'"
    CONTENT_TYPE=$(cat /tmp/completed_event.json | jq -r '.response.output[0].content[0].type')
    echo "- output[0].content[0].type: $CONTENT_TYPE"
    if [ "$CONTENT_TYPE" == "tool_call" ]; then
        echo "✓ Correct: content type is 'tool_call'"
    else
        echo "✗ Wrong: content type should be 'tool_call', got '$CONTENT_TYPE'"
    fi
else
    echo "✗ Wrong: output type should be 'message', got '$OUTPUT_TYPE'"
fi
