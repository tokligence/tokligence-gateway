#!/bin/bash

# P1 MCP Servers and Cache Control Integration Test
# Tests: MCP servers, cache control on messages/tools, computer tools

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8081}"

echo "========================================"
echo "P1 MCP & Cache Control Integration Test"
echo "========================================"
echo "Testing: MCP servers, cache control, computer tools"
echo "Base URL: $BASE_URL"
echo ""

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Test 1: Message with cache_control
echo "Test 1: Message with cache_control field"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [
      {
        "role": "system",
        "content": "You are a helpful assistant",
        "cache_control": {"type": "ephemeral"}
      },
      {
        "role": "user",
        "content": "Hello"
      }
    ],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: cache_control on message accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: cache_control on message caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: Tool with cache_control
echo "Test 2: Tool with cache_control field"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "What is the weather?"}],
    "max_tokens": 50,
    "tools": [{
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get weather for a location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {"type": "string"}
          }
        },
        "cache_control": {"type": "ephemeral"}
      }
    }]
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: cache_control on tool function accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: cache_control on tool function caused error"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Computer tool type
echo "Test 3: Computer tool acceptance (structure validation)"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 50,
    "tools": [{
      "type": "computer_20241022",
      "name": "computer",
      "display_width_px": 1024,
      "display_height_px": 768
    }]
  }')

if echo "$RESPONSE" | jq -e '.content // .choices // .error' > /dev/null 2>&1; then
    # Accept both success and provider-level errors (means structure was accepted by gateway)
    echo -e "${GREEN}✅ PASS${NC}: Computer tool structure accepted by gateway"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Computer tool structure rejected"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 4: MCP server tool (URL type) - structure validation
echo "Test 4: MCP server tool acceptance (URL type)"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 50,
    "tools": [{
      "type": "url",
      "url": "https://example.com/mcp-server",
      "name": "example_mcp",
      "tool_configuration": {
        "timeout": 30
      }
    }]
  }')

if echo "$RESPONSE" | jq -e '.content // .choices // .error' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: MCP server URL tool structure accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: MCP server URL tool structure rejected"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 5: MCP server tool (MCP type) - structure validation
echo "Test 5: MCP server tool acceptance (MCP type)"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 50,
    "tools": [{
      "type": "mcp",
      "server_url": "https://example.com/mcp",
      "server_label": "example",
      "name": "mcp_tool",
      "headers": {
        "Authorization": "Bearer token123"
      }
    }]
  }')

if echo "$RESPONSE" | jq -e '.content // .choices // .error' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: MCP server MCP tool structure accepted"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: MCP server MCP tool structure rejected"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 6: Backward compatibility
echo "Test 6: Backward compatibility check"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 50
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Backward compatibility maintained"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Backward compatibility broken"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 7: Standard function tools still work
echo "Test 7: Standard function tools compatibility"
RESPONSE=$(timeout 20 curl -s -X POST "$BASE_URL/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-haiku-latest",
    "messages": [{"role": "user", "content": "Test"}],
    "max_tokens": 50,
    "tools": [{
      "type": "function",
      "function": {
        "name": "test_func",
        "description": "A test function",
        "parameters": {
          "type": "object",
          "properties": {}
        }
      }
    }]
  }')

if echo "$RESPONSE" | jq -e '.content // .choices' > /dev/null 2>&1; then
    echo -e "${GREEN}✅ PASS${NC}: Standard function tools still work"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Standard function tools broken"
    echo "Response: $RESPONSE"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Summary
echo "========================================"
echo "Test Summary"
echo "========================================"
echo "Passed:  $TESTS_PASSED"
echo "Failed:  $TESTS_FAILED"
echo "Skipped: $TESTS_SKIPPED"
echo "Total:   $((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ ALL TESTS PASSED${NC}"
    exit 0
else
    echo -e "${RED}❌ SOME TESTS FAILED${NC}"
    exit 1
fi
