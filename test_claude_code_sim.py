#!/usr/bin/env python3
"""
Simulate Claude Code behavior with repeated tool calls in the same conversation turn.
This tests the session-based deduplication functionality.
"""

import json
import time
import requests
from datetime import datetime

# Configuration
GATEWAY_URL = "http://localhost:8081/v1/chat/completions"
HEADERS = {
    "Content-Type": "application/json",
    "Authorization": "Bearer test-key",
    "anthropic-version": "2023-06-01",
    "X-User-ID": "test-user-session"
}

def create_claude_request(iteration=1):
    """Create a request simulating Claude Code accumulating history."""
    messages = [
        {
            "role": "user",
            "content": "Please read the test.txt file and analyze its contents"
        }
    ]

    # Add tool uses for each iteration (simulating accumulation)
    for i in range(iteration):
        messages.extend([
            {
                "role": "assistant",
                "content": [
                    {
                        "type": "text",
                        "text": f"I'll read the file for you (attempt {i+1})."
                    },
                    {
                        "type": "tool_use",
                        "id": f"toolu_read_{i+1}_{int(time.time()*1000)}",
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
                        "tool_use_id": f"toolu_read_{i+1}_{int(time.time()*1000)}",
                        "content": "File contents: Hello World from test.txt"
                    }
                ]
            }
        ])

    # Final assistant message trying another tool
    messages.append({
        "role": "assistant",
        "content": [
            {
                "type": "text",
                "text": "The file contains 'Hello World'. Let me check if there are other files."
            },
            {
                "type": "tool_use",
                "id": f"toolu_list_{iteration}_{int(time.time()*1000)}",
                "name": "list_files",
                "input": {
                    "directory": "/tmp"
                }
            }
        ]
    })

    return {
        "model": "claude-3-5-sonnet-20241022",
        "messages": messages,
        "max_tokens": 1000,
        "stream": False
    }

def send_request(request_data, description):
    """Send a request and analyze the response."""
    print(f"\n{'='*60}")
    print(f"Test: {description}")
    print(f"Time: {datetime.now().strftime('%H:%M:%S.%f')[:-3]}")
    print(f"Request has {len(request_data['messages'])} messages")

    try:
        response = requests.post(GATEWAY_URL, headers=HEADERS, json=request_data, timeout=10)

        if response.status_code == 200:
            data = response.json()

            # Check for tool_calls in response
            if 'choices' in data and len(data['choices']) > 0:
                message = data['choices'][0].get('message', {})
                tool_calls = message.get('tool_calls', [])

                print(f"✅ Response received")
                print(f"   Tool calls returned: {len(tool_calls)}")

                if tool_calls:
                    for tc in tool_calls:
                        func = tc.get('function', {})
                        print(f"   - {func.get('name', 'unknown')} (id: {tc.get('id', 'unknown')[:20]}...)")
                else:
                    content = message.get('content', '')
                    if content:
                        print(f"   Content: {content[:100]}...")

                return True, len(tool_calls)
            else:
                print(f"❌ Unexpected response structure")
                print(json.dumps(data, indent=2)[:500])
                return False, 0
        else:
            print(f"❌ HTTP {response.status_code}")
            print(f"   Response: {response.text[:200]}")
            return False, 0

    except Exception as e:
        print(f"❌ Error: {str(e)}")
        return False, 0

def main():
    print("="*60)
    print("Claude Code Session Deduplication Test")
    print("Testing tool call deduplication across multiple API calls")
    print("="*60)

    results = []

    # Simulate Claude Code making multiple requests in the same conversation
    for i in range(1, 4):
        request = create_claude_request(iteration=i)
        success, tool_count = send_request(
            request,
            f"Request {i}: Accumulated {i} read_file calls + 1 list_files"
        )
        results.append((i, success, tool_count))

        # Small delay between requests
        time.sleep(0.5)

    # Summary
    print("\n" + "="*60)
    print("SUMMARY")
    print("="*60)

    for i, success, tool_count in results:
        status = "✅" if success else "❌"
        print(f"Request {i}: {status} - {tool_count} tool calls returned")

    # Check if deduplication is working
    if all(success for _, success, _ in results):
        # Ideally, later requests should return fewer or no duplicate tools
        if results[-1][2] < results[0][2]:
            print("\n✅ Deduplication appears to be working!")
            print("   Later requests returned fewer tool calls")
        elif all(tc == 0 for _, _, tc in results[1:]):
            print("\n✅ Strong deduplication - no tools returned after first request")
        else:
            print("\n⚠️  Tool calls still being returned in later requests")
            print("   This may indicate deduplication needs tuning")

    # Check logs
    print("\n" + "="*60)
    print("LOG CHECK")
    print("To verify deduplication, check the gateway logs:")
    print("  grep -i 'openai.bridge\\|suppress duplicate\\|suppress repeated' logs/dev-gatewayd.log | tail -50")

if __name__ == "__main__":
    main()
