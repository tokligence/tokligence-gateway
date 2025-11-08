#!/bin/bash
# Verify that tool call responses have "incomplete" status (not "completed")

echo "=== Testing tool call response status ==="
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
}' 2>&1 > /tmp/status_test.txt

echo "Extracting response.completed event..."
grep -A 1 'event: response.completed' /tmp/status_test.txt | tail -1 | sed 's/^data: //' > /tmp/status_response.json

echo ""
echo "=== Verification ==="
echo ""

RESPONSE_STATUS=$(cat /tmp/status_response.json | jq -r '.response.status')
OUTPUT_TYPE=$(cat /tmp/status_response.json | jq -r '.response.output[0].type')

echo "1. Response status: $RESPONSE_STATUS"
if [ "$RESPONSE_STATUS" == "incomplete" ]; then
    echo "   ✓ Correct: status is 'incomplete' (client needs to execute tool)"
else
    echo "   ✗ Wrong: expected 'incomplete', got '$RESPONSE_STATUS'"
fi

echo ""
echo "2. Output type: $OUTPUT_TYPE"
if [ "$OUTPUT_TYPE" == "function_call" ]; then
    echo "   ✓ Correct: output type is 'function_call'"
else
    echo "   ✗ Wrong: expected 'function_call', got '$OUTPUT_TYPE'"
fi

echo ""
echo "3. Stream termination:"
if grep -q "data: \[DONE\]" /tmp/status_test.txt; then
    echo "   ✓ [DONE] marker present"
else
    echo "   ✗ [DONE] marker missing"
fi

echo ""
echo "=== Full response object ==="
cat /tmp/status_response.json | jq '.response'
