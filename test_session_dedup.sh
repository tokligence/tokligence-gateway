#!/bin/bash

# Test script for session-based tool call deduplication
# This script simulates Claude Code making repeated requests with the same tools

set -e

echo "=== Session Deduplication Test ==="
echo "Testing tool call deduplication with session management"
echo

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Gateway endpoint
GATEWAY_URL="http://localhost:8081/v1/chat/completions"
API_KEY="test-key-12345"

# Create a test request with tool calls (simulating Claude Code behavior)
create_test_request() {
    local user_id="$1"
    local message="$2"
    local tools="$3"

    cat << EOF
{
    "model": "claude-3-5-sonnet-20241022",
    "messages": [
        {
            "role": "user",
            "content": "$message"
        },
        {
            "role": "assistant",
            "content": [
                {
                    "type": "text",
                    "text": "I'll help you with that."
                },
                {
                    "type": "tool_use",
                    "id": "tool_$(date +%s%N)",
                    "name": "read_file",
                    "input": {
                        "path": "/tmp/test.txt"
                    }
                }
            ]
        },
        {
            "role": "user",
            "content": [
                {
                    "type": "tool_result",
                    "tool_use_id": "tool_$(date +%s%N)",
                    "content": "File contents: Hello World"
                }
            ]
        },
        {
            "role": "assistant",
            "content": [
                {
                    "type": "text",
                    "text": "The file contains 'Hello World'. Let me check another file."
                },
                {
                    "type": "tool_use",
                    "id": "tool_new_$(date +%s%N)",
                    "name": "read_file",
                    "input": {
                        "path": "/tmp/test2.txt"
                    }
                }
            ]
        }
    ],
    "max_tokens": 1000,
    "stream": false
}
EOF
}

# Function to send request and check response
send_request() {
    local request_data="$1"
    local test_name="$2"

    echo -e "${YELLOW}Test: $test_name${NC}"

    response=$(curl -s -X POST "$GATEWAY_URL" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer $API_KEY" \
        -H "X-User-ID: test-user-123" \
        -d "$request_data")

    # Check if response contains expected patterns
    if echo "$response" | jq -e '.choices[0].message.tool_calls' > /dev/null 2>&1; then
        tool_count=$(echo "$response" | jq '.choices[0].message.tool_calls | length')
        echo -e "${GREEN}✓ Response contains $tool_count tool calls${NC}"

        # Extract tool names for display
        echo "$response" | jq -r '.choices[0].message.tool_calls[].function.name' | while read tool_name; do
            echo "  - Tool: $tool_name"
        done
    else
        echo -e "${RED}✗ No tool calls in response${NC}"
    fi

    echo "$response" | jq -C '.'
    echo "---"
}

# Test 1: Send initial request
echo -e "${YELLOW}=== Test 1: Initial Request ===${NC}"
initial_request=$(create_test_request "test-user-123" "Please read some files" "read_file")
send_request "$initial_request" "Initial request with tools"

sleep 1

# Test 2: Send duplicate request (should be deduplicated)
echo -e "${YELLOW}=== Test 2: Duplicate Request (Same Session) ===${NC}"
send_request "$initial_request" "Duplicate request - should show deduplication"

sleep 1

# Test 3: Send request with different user (new session)
echo -e "${YELLOW}=== Test 3: Different User (New Session) ===${NC}"
different_user_request=$(create_test_request "different-user-456" "Please read some files" "read_file")
send_request "$different_user_request" "Different user - new session"

# Check logs for deduplication messages
echo
echo -e "${YELLOW}=== Checking Logs for Deduplication ===${NC}"
if [ -f "logs/dev-gatewayd.log" ]; then
    echo "Recent deduplication log entries:"
    grep -i "openai.bridge\|suppress duplicate\|suppress repeated" logs/dev-gatewayd.log | tail -20 || echo "No deduplication entries found"
fi

echo
echo -e "${GREEN}=== Test Complete ===${NC}"
echo "Check the logs for bridge behavior:"
echo "  tail -f logs/dev-gatewayd.log | grep -i openai.bridge"
