#!/bin/bash
# Test tool call event sequence validation

echo "=== Test 5: SSE Event Sequence Validation ==="
TMPFILE=$(mktemp)

curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "Use the shell tool to run: echo test123"}]}
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
}' 2>&1 > "$TMPFILE"

echo "Checking for required events..."
echo ""

# Check each required event
events=(
  "response.created"
  "response.output_item.added"
  "response.function_call_arguments.delta"
  "response.function_call_arguments.done"
  "response.output_item.done"
  "response.completed"
)

all_present=true
for event in "${events[@]}"; do
  if grep -q "event: $event" "$TMPFILE"; then
    echo "✓ $event"
  else
    echo "✗ $event (MISSING)"
    all_present=false
  fi
done

echo ""
if [ "$all_present" = true ]; then
  echo "✓ All required events present"
  exit 0
else
  echo "✗ Some events are missing"
  echo ""
  echo "Full output:"
  cat "$TMPFILE"
  rm "$TMPFILE"
  exit 1
fi

rm "$TMPFILE"
