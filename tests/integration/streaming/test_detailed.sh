#!/bin/bash
# Detailed streaming tool call validation

TMPFILE=$(mktemp)
echo "=== Detailed Streaming Tool Call Validation ==="
echo ""

curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "Use the shell tool to run: echo test"}]}
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

echo "1. Checking event sequence..."
expected_sequence=(
  "response.created"
  "response.output_item.added"
  "response.function_call_arguments.delta"
  "response.function_call_arguments.done"
  "response.output_item.done"
  "response.completed"
)

prev_seq=-1
seq_ok=true
for event in "${expected_sequence[@]}"; do
  # Get first occurrence sequence number
  seq=$(grep -A1 "event: $event" "$TMPFILE" | grep '"sequence_number"' | head -1 | grep -oP '"sequence_number":\K[0-9]+')
  if [ -n "$seq" ]; then
    if [ "$seq" -gt "$prev_seq" ]; then
      echo "  ✓ $event (seq=$seq)"
      prev_seq=$seq
    else
      echo "  ✗ $event (seq=$seq, expected > $prev_seq)"
      seq_ok=false
    fi
  else
    echo "  ✗ $event (not found or no sequence_number)"
    seq_ok=false
  fi
done
echo ""

echo "2. Checking item_id consistency..."
# Extract all item_ids from delta and done events
item_ids=$(grep -oP '"item_id":"[^"]+' "$TMPFILE" | cut -d'"' -f4 | sort -u)
item_id_count=$(echo "$item_ids" | wc -l)
if [ "$item_id_count" -eq 1 ]; then
  echo "  ✓ All events use same item_id: $item_ids"
else
  echo "  ✗ Multiple item_ids found: $item_id_count"
  echo "$item_ids"
fi
echo ""

echo "3. Checking call_id consistency..."
# Extract all call_ids
call_ids=$(grep -oP '"call_id":"[^"]+' "$TMPFILE" | cut -d'"' -f4 | sort -u)
call_id_count=$(echo "$call_ids" | wc -l)
if [ "$call_id_count" -eq 1 ]; then
  echo "  ✓ All events use same call_id: $call_ids"
else
  echo "  ✗ Multiple call_ids found: $call_id_count"
  echo "$call_ids"
fi
echo ""

echo "4. Checking function name..."
# Check output_item.added and output_item.done have name field
added_name=$(grep -A1 'response.output_item.added' "$TMPFILE" | grep -oP '"name":"[^"]+' | cut -d'"' -f4 | head -1)
done_name=$(grep -A1 'response.output_item.done' "$TMPFILE" | grep -oP '"name":"[^"]+' | cut -d'"' -f4 | head -1)
if [ "$added_name" = "shell" ] && [ "$done_name" = "shell" ]; then
  echo "  ✓ Function name correct in both events: $added_name"
else
  echo "  ✗ Function name mismatch: added=$added_name, done=$done_name"
fi
echo ""

echo "5. Checking status transitions..."
added_status=$(grep -A1 'response.output_item.added' "$TMPFILE" | grep -oP '"status":"[^"]+' | cut -d'"' -f4 | head -1)
done_status=$(grep -A1 'response.output_item.done' "$TMPFILE" | grep -oP '"status":"[^"]+' | cut -d'"' -f4 | head -1)
completed_status=$(grep -A1 'response.completed' "$TMPFILE" | grep -oP '"status":"[^"]+' | cut -d'"' -f4 | tail -1)

if [ "$added_status" = "in_progress" ]; then
  echo "  ✓ output_item.added status: in_progress"
else
  echo "  ✗ output_item.added status: $added_status (expected in_progress)"
fi

if [ "$done_status" = "completed" ]; then
  echo "  ✓ output_item.done status: completed"
else
  echo "  ✗ output_item.done status: $done_status (expected completed)"
fi

if [ "$completed_status" = "completed" ]; then
  echo "  ✓ response.completed status: completed"
else
  echo "  ✗ response.completed status: $completed_status (expected completed)"
fi
echo ""

echo "6. Checking arguments accumulation..."
# Get final arguments from done event
done_args=$(grep -A1 'response.function_call_arguments.done' "$TMPFILE" | grep -oP '"arguments":"[^"]+' | cut -d'"' -f4)
# Get final arguments from completed event
completed_args=$(grep -A1 'response.completed' "$TMPFILE" | tail -20 | grep -oP '"arguments":"[^"]+' | cut -d'"' -f4)

if [ -n "$done_args" ]; then
  echo "  ✓ function_call_arguments.done has arguments: $done_args"
else
  echo "  ✗ function_call_arguments.done missing arguments"
fi

if [ "$done_args" = "$completed_args" ]; then
  echo "  ✓ Arguments match in done and completed events"
else
  echo "  ✗ Arguments mismatch:"
  echo "    done: $done_args"
  echo "    completed: $completed_args"
fi
echo ""

echo "7. Checking type fields..."
added_type=$(grep -A1 'response.output_item.added' "$TMPFILE" | grep -oP '"type":"function_call"' | wc -l)
done_type=$(grep -A1 'response.output_item.done' "$TMPFILE" | grep -oP '"type":"function_call"' | wc -l)
completed_type=$(grep 'response.completed' "$TMPFILE" -A50 | grep -oP '"type":"function_call"' | wc -l)

if [ "$added_type" -ge 1 ]; then
  echo "  ✓ output_item.added has type=function_call"
else
  echo "  ✗ output_item.added missing type=function_call"
fi

if [ "$done_type" -ge 1 ]; then
  echo "  ✓ output_item.done has type=function_call"
else
  echo "  ✗ output_item.done missing type=function_call"
fi

if [ "$completed_type" -ge 1 ]; then
  echo "  ✓ response.completed output has type=function_call"
else
  echo "  ✗ response.completed output missing type=function_call"
fi
echo ""

echo "=========================================="
echo "Full SSE Output:"
echo "=========================================="
cat "$TMPFILE"
echo ""

rm "$TMPFILE"
