#!/bin/bash
# Test complete tool call flow with submit_tool_outputs

set -e

BASE_URL="${1:-http://localhost:8081}"
API_KEY="${2:-test}"

echo "=== Testing Complete Tool Call Flow ==="
echo ""

# Step 1: Start the initial request in background and save to temp file
TEMP_FILE=$(mktemp)
echo "Starting initial request (saving to $TEMP_FILE)..."

curl -s -N -X POST "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d @/tmp/test_tool_stream_req.json > "$TEMP_FILE" 2>&1 &

CURL_PID=$!
echo "curl PID: $CURL_PID"

# Wait for initial response with tool call
sleep 3

# Check if we got the response
if [ ! -s "$TEMP_FILE" ]; then
  echo "ERROR: No response received"
  kill $CURL_PID 2>/dev/null || true
  rm -f "$TEMP_FILE"
  exit 1
fi

# Extract response ID
RESPONSE_ID=$(grep -oP '"id":"resp_[0-9]+"' "$TEMP_FILE" | head -1 | grep -oP 'resp_[0-9]+' || echo "")

if [ -z "$RESPONSE_ID" ]; then
  echo "ERROR: Could not extract response ID"
  cat "$TEMP_FILE"
  kill $CURL_PID 2>/dev/null || true
  rm -f "$TEMP_FILE"
  exit 1
fi

echo "✓ Got response ID: $RESPONSE_ID"

# Extract tool call ID
TOOL_CALL_ID=$(grep -oP '"call_id":"[^"]+"' "$TEMP_FILE" | head -1 | grep -oP 'call_[0-9]+' || echo "")

if [ -z "$TOOL_CALL_ID" ]; then
  echo "ERROR: Could not extract tool call ID"
  cat "$TEMP_FILE"
  kill $CURL_PID 2>/dev/null || true
  rm -f "$TEMP_FILE"
  exit 1
fi

echo "✓ Got tool call ID: $TOOL_CALL_ID"

# Check if required_action was sent
if grep -q "response.required_action" "$TEMP_FILE"; then
  echo "✓ Got required_action event"
else
  echo "ERROR: Did not get required_action"
  cat "$TEMP_FILE"
  kill $CURL_PID 2>/dev/null || true
  rm -f "$TEMP_FILE"
  exit 1
fi

# Check if response.completed with incomplete was sent
if grep -q '"status":"incomplete"' "$TEMP_FILE"; then
  echo "✓ Got response.completed with status=incomplete"
else
  echo "ERROR: Did not get incomplete status"
  kill $CURL_PID 2>/dev/null || true
  rm -f "$TEMP_FILE"
  exit 1
fi

echo ""
echo "=== Submitting Tool Outputs ==="

# Step 2: Submit tool outputs
TOOL_OUTPUT_PAYLOAD=$(cat <<EOF
{
  "tool_outputs": [
    {
      "tool_call_id": "$TOOL_CALL_ID",
      "output": "File created successfully"
    }
  ]
}
EOF
)

echo "Submitting tool outputs to /v1/responses/$RESPONSE_ID/submit_tool_outputs..."
SUBMIT_RESPONSE=$(curl -s -X POST "$BASE_URL/v1/responses/$RESPONSE_ID/submit_tool_outputs" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $API_KEY" \
  -d "$TOOL_OUTPUT_PAYLOAD")

echo "Submit response: $SUBMIT_RESPONSE"

if echo "$SUBMIT_RESPONSE" | grep -q "accepted"; then
  echo "✓ Tool outputs accepted"
else
  echo "ERROR: Tool outputs not accepted"
  echo "$SUBMIT_RESPONSE"
  kill $CURL_PID 2>/dev/null || true
  rm -f "$TEMP_FILE"
  exit 1
fi

# Wait for stream to continue
echo ""
echo "=== Waiting for stream to continue ==="
sleep 3

# Check if curl is still running
if ps -p $CURL_PID > /dev/null 2>&1; then
  echo "✓ Stream still open (good)"
else
  echo "✗ Stream closed (may have completed)"
fi

# Wait a bit more for completion
sleep 2

# Check the complete output
cat "$TEMP_FILE" | tail -20

# Cleanup
kill $CURL_PID 2>/dev/null || true
rm -f "$TEMP_FILE"

echo ""
echo "=== Test Complete ==="
