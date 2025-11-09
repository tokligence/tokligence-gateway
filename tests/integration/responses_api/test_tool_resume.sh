#!/usr/bin/env bash
set -euo pipefail

# Session-aware SSE validation for /v1/responses.
# Flow:
#   1. Start a streaming Responses request that is guaranteed to emit a tool call.
#   2. Wait for required_action + capture response_id + tool_call_id + arguments.
#   3. Execute the tool locally (write file) and POST /v1/responses/{id}/submit_tool_outputs.
#   4. Verify the same SSE connection finishes with status=completed and [DONE].
#
# Requirements:
#   - Gateway running at http://localhost:8081 with Anthropic routing and valid API keys (.env already provides them).
#   - jq, python3, curl, timeout.
#   - Optional TOKLIGENCE_API_KEY env var if auth is enabled (skipped when auth_disabled=true).
# Usage:
#   tests/test_responses_tool_resume.sh [request_json]

REQ_JSON=${1:-tests/fixtures/tool_calls/basic_request.json}
BASE_URL=${BASE_URL:-http://localhost:8081}
AUTH_HEADER=()
if [[ -n "${TOKLIGENCE_API_KEY:-}" ]]; then
  AUTH_HEADER=(-H "Authorization: Bearer ${TOKLIGENCE_API_KEY}")
fi

if [[ ! -f "$REQ_JSON" ]]; then
  echo "Fixture not found: $REQ_JSON" >&2
  exit 1
fi

LOG=$(mktemp)
STREAM_PID=""
cleanup() {
  if [[ -n "$STREAM_PID" ]] && kill -0 "$STREAM_PID" 2>/dev/null; then
    kill "$STREAM_PID" 2>/dev/null || true
  fi
  rm -f "$LOG"
}
trap cleanup EXIT

curl -sS -N "$BASE_URL/v1/responses" \
  -H "Content-Type: application/json" \
  "${AUTH_HEADER[@]}" \
  -d @"$REQ_JSON" >"$LOG" &
STREAM_PID=$!
echo "[stream] started PID=$STREAM_PID log=$LOG"

wait_for_pattern() {
  local pattern=$1 timeout_s=${2:-60}
  for ((i=0;i<timeout_s;i++)); do
    if grep -q "$pattern" "$LOG" 2>/dev/null; then
      return 0
    fi
    if ! kill -0 "$STREAM_PID" 2>/dev/null; then
      break
    fi
    sleep 1
  done
  return 1
}

if ! wait_for_pattern 'required_action' 90; then
  echo "Timed out waiting for required_action" >&2
  tail -n 200 "$LOG" >&2
  exit 1
fi

PARSE=$(python3 - "$LOG" <<'PY'
import json, sys
log = sys.argv[1]
resp_id = None
call_id = None
args_json = None
for line in open(log):
    line=line.strip()
    if not line.startswith('data: '):
        continue
    payload = line[6:]
    if payload == '[DONE]':
        continue
    try:
        event = json.loads(payload)
    except json.JSONDecodeError:
        continue
    if event.get('type') == 'response.created':
        resp_id = event.get('response', {}).get('id')
    if event.get('type') == 'response.completed':
        resp = event.get('response', {})
        required = resp.get('required_action') or {}
        sto = required.get('submit_tool_outputs', {})
        calls = sto.get('tool_calls') or []
        if calls:
            call = calls[0]
            call_id = call.get('id')
            fn = call.get('function') or {}
            args_json = fn.get('arguments')
            break
if not resp_id or not call_id or not args_json:
    print('PARSE_ERROR')
    sys.exit(1)
print(resp_id)
print(call_id)
print(args_json)
PY
) || true
if [[ "$PARSE" == "PARSE_ERROR" || -z "$PARSE" ]]; then
  echo "Failed to parse response metadata" >&2
  tail -n 200 "$LOG" >&2
  exit 1
fi
RESP_ID=$(echo "$PARSE" | sed -n '1p')
CALL_ID=$(echo "$PARSE" | sed -n '2p')
TOOL_ARGS_JSON=$(echo "$PARSE" | sed -n '3p')

echo "[stream] resp_id=$RESP_ID call_id=$CALL_ID"

temp_args=$(mktemp)
printf '%s' "$TOOL_ARGS_JSON" > "$temp_args"
TOOL_RESULT=$(python3 - "$temp_args" <<'PY'
import json, sys, pathlib
args = json.load(open(sys.argv[1]))
path = pathlib.Path(args['path'])
path.parent.mkdir(parents=True, exist_ok=True)
path.write_text(args['contents'])
print(json.dumps({"status":"file_written","path":str(path)}))
PY
)
rm -f "$temp_args"

data_payload=$(jq -n --arg id "$CALL_ID" --arg output "$TOOL_RESULT" '{tool_outputs:[{tool_call_id:$id, output:$output}]}' )
resp=$(curl -sS -X POST "$BASE_URL/v1/responses/$RESP_ID/submit_tool_outputs" \
  -H "Content-Type: application/json" "${AUTH_HEADER[@]}" \
  -d "$data_payload")
if ! echo "$resp" | grep -q 'accepted'; then
  echo "submit_tool_outputs failed: $resp" >&2
  exit 1
fi

echo "[submit] tool outputs accepted"

if ! wait_for_pattern '"status":"completed"' 90; then
  echo "Timed out waiting for final completion" >&2
  tail -n 200 "$LOG" >&2
  exit 1
fi
if ! wait_for_pattern 'data: \[DONE\]' 30; then
  echo "[DONE] marker missing" >&2
  exit 1
fi

wait "$STREAM_PID" || true

echo "=== Final response ==="
python3 - "$LOG" <<'PY'
import json, sys
log = sys.argv[1]
final = None
for line in open(log):
    if not line.startswith('data: '):
        continue
    payload = line[6:].strip()
    if payload == '[DONE]':
        continue
    try:
        data = json.loads(payload)
    except json.JSONDecodeError:
        continue
    if data.get('type') == 'response.completed':
        final = data
print(json.dumps(final, indent=2))
PY
