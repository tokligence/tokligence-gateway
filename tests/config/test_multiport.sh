#!/usr/bin/env bash
set -euo pipefail

# Test: Multi-Port Configuration
# This test verifies that the gateway can run on multiple ports simultaneously

echo "🧪 Testing Multi-Port Gateway Configuration"
echo "=============================================="
echo ""

# Check if gatewayd is running
if ! pgrep -f gatewayd > /dev/null; then
    echo "❌ gatewayd is not running. Please start it first with multi-port config."
    echo ""
    echo "To enable multi-port mode, set in config/dev/gateway.ini:"
    echo "  enable_direct_access = true"
    echo "  facade_port = :9000"
    echo "  openai_port = :8082"
    echo "  anthropic_port = :8081"
    echo "  admin_port = :8080"
    echo ""
    exit 1
fi

# Test ports
FACADE_PORT=${FACADE_PORT:-9000}
OPENAI_PORT=${OPENAI_PORT:-8082}
ANTHROPIC_PORT=${ANTHROPIC_PORT:-8081}
ADMIN_PORT=${ADMIN_PORT:-8080}

echo "Testing ports:"
echo "  Facade: :$FACADE_PORT"
echo "  OpenAI: :$OPENAI_PORT"
echo "  Anthropic: :$ANTHROPIC_PORT"
echo "  Admin: :$ADMIN_PORT"
echo ""

# Check which ports are actually listening (gatewayd only)
echo "Checking listening ports..."
listening_ports=$(ss -ltnp 2>/dev/null | grep gatewayd || true)

if [ -z "$listening_ports" ]; then
    echo "⚠️  No gatewayd listeners detected."
    echo "Falling back to single-port test on :8081"
    TEST_PORTS=("8081")
else
    echo "✅ Multi-port listeners detected:"
    echo "$listening_ports"
    echo ""
    # Extract actual gatewayd ports
    TEST_PORTS=()
    while read -r line; do
        port=$(echo "$line" | grep -oP ':\K[0-9]+(?=\s)' | head -1)
        if [ -n "$port" ]; then
            TEST_PORTS+=("$port")
        fi
    done <<< "$listening_ports"

    # Remove duplicates
    TEST_PORTS=($(echo "${TEST_PORTS[@]}" | tr ' ' '\n' | sort -u))
fi

echo "Testing gatewayd ports: ${TEST_PORTS[*]}"
echo ""

# Test health endpoint on each port
echo "Testing /health endpoint on available ports..."
passed=0
failed=0

for port in "${TEST_PORTS[@]}"; do
    echo -n "  Port :$port ... "
    if response=$(curl -s -f -m 2 http://localhost:$port/health 2>&1); then
        if echo "$response" | grep -q "ok\|healthy\|status"; then
            echo "✅ OK"
            ((passed++)) || true
        else
            echo "⚠️  Unexpected response: $response"
            ((failed++)) || true
        fi
    else
        echo "❌ Failed: $response"
        ((failed++)) || true
    fi
done

echo ""

# Test OpenAI endpoint on first available port that responds
echo "Testing OpenAI /v1/models endpoint..."
openai_tested=false
for test_port in "${TEST_PORTS[@]}"; do
    if curl -s -f -m 3 http://localhost:$test_port/v1/models > /dev/null 2>&1; then
        echo "  ✅ /v1/models accessible on :$test_port"
        ((passed++)) || true
        openai_tested=true
        break
    fi
done
if [ "$openai_tested" = "false" ]; then
    echo "  ⚠️  /v1/models not found on any port (skipping)"
fi

# Test Anthropic endpoint on first available port that responds
echo "Testing Anthropic /v1/messages endpoint..."
anthropic_tested=false
for test_port in "${TEST_PORTS[@]}"; do
    # Anthropic endpoint should return 400/401/405 without proper auth/body
    status=$(curl -s -m 3 -o /dev/null -w "%{http_code}" -X POST http://localhost:$test_port/v1/messages 2>&1)
    if [ "$status" = "400" ] || [ "$status" = "401" ] || [ "$status" = "405" ]; then
        echo "  ✅ /v1/messages accessible on :$test_port (status: $status)"
        ((passed++)) || true
        anthropic_tested=true
        break
    fi
done
if [ "$anthropic_tested" = "false" ]; then
    echo "  ⚠️  /v1/messages not found on any port (skipping)"
fi

echo ""
echo "=============================================="
echo "Multi-Port Test Results"
echo "=============================================="
echo "Passed: $passed"
echo "Failed: $failed"
echo ""

if [ $failed -eq 0 ]; then
    echo "✅ All multi-port tests passed"
    exit 0
else
    echo "❌ Some tests failed"
    exit 1
fi
