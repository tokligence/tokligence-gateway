#!/bin/bash
# Check what format OpenAI actually uses for tool calls in Response API

echo "=== Testing against real OpenAI Response API (if key available) ==="
echo ""

if [ -z "$OPENAI_API_KEY" ]; then
    echo "❌ OPENAI_API_KEY not set. Cannot test against real OpenAI."
    echo ""
    echo "To test, set OPENAI_API_KEY and run:"
    echo "  export OPENAI_API_KEY=sk-..."
    echo "  $0"
    exit 1
fi

echo "Testing with streaming=true to see SSE event format..."
echo ""

curl -s -N https://api.openai.com/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -d '{
  "model": "gpt-4o-mini",
  "input": [{
    "role": "user",
    "content": [{
      "type": "input_text",
      "text": "Use the get_weather tool to check weather in San Francisco"
    }]
  }],
  "tools": [{
    "type": "function",
    "name": "get_weather",
    "description": "Get weather for a location",
    "parameters": {
      "type": "object",
      "properties": {
        "location": {"type": "string"}
      },
      "required": ["location"]
    }
  }],
  "tool_choice": true,
  "stream": true
}' 2>&1 | tee /tmp/openai_tool_stream.txt

echo ""
echo ""
echo "=== Extracting response.completed event ==="
grep 'response.completed' -A1 /tmp/openai_tool_stream.txt | tail -1 | jq '.' > /tmp/openai_completed.json

echo ""
echo "Output structure:"
cat /tmp/openai_completed.json | jq '.response.output[0]' 2>/dev/null || echo "No output found"

echo ""
echo "Full completed event:"
cat /tmp/openai_completed.json
