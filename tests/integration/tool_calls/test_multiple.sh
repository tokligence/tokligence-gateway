#!/bin/bash
# Tool call test with multiple tools available

echo "=== Test 3: Multiple Tools Available ==="
curl -s -N http://localhost:8081/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer test" \
  -d '{
  "model": "claude-3-5-sonnet-20241022",
  "input": [
    {"role": "user", "content": [{"type": "input_text", "text": "Get the current weather in San Francisco"}]}
  ],
  "tools": [
    {
      "type": "function",
      "name": "get_weather",
      "description": "Get current weather for a location",
      "parameters": {
        "type": "object",
        "properties": {
          "location": {"type": "string", "description": "City name"},
          "unit": {"type": "string", "enum": ["celsius", "fahrenheit"], "default": "fahrenheit"}
        },
        "required": ["location"]
      }
    },
    {
      "type": "function",
      "name": "search_web",
      "description": "Search the web for information",
      "parameters": {
        "type": "object",
        "properties": {
          "query": {"type": "string"}
        },
        "required": ["query"]
      }
    }
  ],
  "tool_choice": true,
  "stream": true
}' 2>&1 | grep -E "^(event:|data:)" | head -25

echo ""
echo "Expected: tool call to get_weather with location=San Francisco"
echo ""
