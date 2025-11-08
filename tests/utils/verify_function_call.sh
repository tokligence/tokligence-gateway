#!/bin/bash
# Verify that tool calls use the correct flat function_call format (not wrapped in message)

echo "=== Verifying flat function_call format in response.completed ===="
echo ""

curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [{
    "role": "user",
    "content": [{
      "type": "input_text",
      "text": "Use shell to write hello to test.txt"
    }]
  }],
  "tools": [{
    "type": "function",
    "name": "shell",
    "description": "Execute command",
    "parameters": {
      "type": "object",
      "properties": {
        "command": {"type": "string"}
      },
      "required": ["command"]
    }
  }],
  "tool_choice": true,
  "stream": true
}' 2>&1 | tee /tmp/flat_test.txt

echo ""
echo "=== Checking format ==="
echo ""

# Extract response.completed
grep 'response.completed' -A1 /tmp/flat_test.txt | tail -1 > /tmp/flat_completed.json

echo "1. Checking output[0] type:"
OUTPUT_TYPE=$(cat /tmp/flat_completed.json | jq -r '.response.output[0].type')
echo "   output[0].type = $OUTPUT_TYPE"

if [ "$OUTPUT_TYPE" == "function_call" ]; then
    echo "   ✓ Correct: flat function_call (not wrapped in message)"
else
    echo "   ✗ Wrong: expected 'function_call', got '$OUTPUT_TYPE'"
fi

echo ""
echo "2. Checking for required fields:"
HAS_ID=$(cat /tmp/flat_completed.json | jq '.response.output[0].id')
HAS_NAME=$(cat /tmp/flat_completed.json | jq '.response.output[0].name')
HAS_CALL_ID=$(cat /tmp/flat_completed.json | jq '.response.output[0].call_id')
HAS_ARGS=$(cat /tmp/flat_completed.json | jq '.response.output[0].arguments')
HAS_STATUS=$(cat /tmp/flat_completed.json | jq '.response.output[0].status')

echo "   - id: $([ "$HAS_ID" != "null" ] && echo "✓" || echo "✗")"
echo "   - name: $([ "$HAS_NAME" != "null" ] && echo "✓" || echo "✗")"
echo "   - call_id: $([ "$HAS_CALL_ID" != "null" ] && echo "✓" || echo "✗")"
echo "   - arguments: $([ "$HAS_ARGS" != "null" ] && echo "✓" || echo "✗")"
echo "   - status: $([ "$HAS_STATUS" != "null" ] && echo "✓" || echo "✗")"

echo ""
echo "3. Checking for [DONE] marker:"
if grep -q "data: \[DONE\]" /tmp/flat_test.txt; then
    echo "   ✓ [DONE] marker present"
else
    echo "   ✗ [DONE] marker missing"
fi

echo ""
echo "Full output[0] structure:"
cat /tmp/flat_completed.json | jq '.response.output[0]'
