#!/usr/bin/env bash
set -euo pipefail

# Test /v1/responses tool-calls against the running gateway on :8081
# Usage:
#   ./scripts/test_responses_tools.sh claude-3-5-sonnet-20241022

MODEL=${1:-claude-3-5-sonnet-20241022}

echo "[1/2] Request tool call (stream=true)"
curl -N -sS -X POST http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$MODEL"'",
    "instructions": "Call the tool if needed.",
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a city",
        "parameters": {"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}
      }
    }],
    "tool_choice": "required",
    "input": "What is the weather in San Francisco?",
    "stream": true
  }' | head -n 40

echo
echo "[2/2] Provide tool result (non-stream)"
curl -sS -X POST http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'"$MODEL"'",
    "messages": [
      {"role": "user", "content": "What is the weather in San Francisco?"},
      {"role": "assistant", "tool_calls": [
        {"id":"toolu_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"San Francisco\"}"}}
      ]},
      {"role": "tool", "tool_call_id":"toolu_1", "content": "{\"temp_c\":20,\"cond\":\"sunny\"}"}
    ],
    "max_output_tokens": 128
  }' | jq .

echo "Done."

