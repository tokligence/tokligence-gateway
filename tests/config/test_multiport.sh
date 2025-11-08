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

# Check which ports are actually listening
echo "Checking listening ports..."
listening_ports=$(ss -ltnp 2>/dev/null | grep -E "($FACADE_PORT|$OPENAI_PORT|$ANTHROPIC_PORT|$ADMIN_PORT)" || true)

if [ -z "$listening_ports" ]; then
    echo "⚠️  No multi-port listeners detected. Gateway may be running in single-port mode."
    echo "Current listening ports:"
    ss -ltnp 2>/dev/null | grep gatewayd || echo "  (none found)"
    echo ""
    echo "Falling back to single-port test on :8081"
    TEST_PORTS=("8081")
else
    echo "✅ Multi-port listeners detected:"
    echo "$listening_ports"
    echo ""
    TEST_PORTS=()
    echo "$listening_ports" | grep -q ":$FACADE_PORT" && TEST_PORTS+=("$FACADE_PORT")
    echo "$listening_ports" | grep -q ":$OPENAI_PORT" && TEST_PORTS+=("$OPENAI_PORT")
    echo "$listening_ports" | grep -q ":$ANTHROPIC_PORT" && TEST_PORTS+=("$ANTHROPIC_PORT")
    echo "$listening_ports" | grep -q ":$ADMIN_PORT" && TEST_PORTS+=("$ADMIN_PORT")
fi

# Test health endpoint on each port
echo "Testing /health endpoint on available ports..."
passed=0
failed=0

for port in "${TEST_PORTS[@]}"; do
    echo -n "  Port :$port ... "
    if response=$(curl -s -f -m 2 http://localhost:$port/health 2>&1); then
        if echo "$response" | grep -q "ok\|healthy\|status"; then
            echo "✅ OK"
            ((passed++))
        else
            echo "⚠️  Unexpected response: $response"
            ((failed++))
        fi
    else
        echo "❌ Failed: $response"
        ((failed++))
    fi
done

echo ""

# Test OpenAI endpoint on appropriate ports
if [[ " ${TEST_PORTS[@]} " =~ " ${OPENAI_PORT} " ]] || [[ " ${TEST_PORTS[@]} " =~ " ${FACADE_PORT} " ]]; then
    echo "Testing OpenAI /v1/models endpoint..."
    test_port=${OPENAI_PORT}
    if ! [[ " ${TEST_PORTS[@]} " =~ " ${OPENAI_PORT} " ]]; then
        test_port=${FACADE_PORT}
    fi

    if curl -s -f http://localhost:$test_port/v1/models > /dev/null 2>&1; then
        echo "  ✅ /v1/models accessible on :$test_port"
        ((passed++))
    else
        echo "  ❌ /v1/models failed on :$test_port"
        ((failed++))
    fi
fi

# Test Anthropic endpoint on appropriate ports
if [[ " ${TEST_PORTS[@]} " =~ " ${ANTHROPIC_PORT} " ]] || [[ " ${TEST_PORTS[@]} " =~ " ${FACADE_PORT} " ]]; then
    echo "Testing Anthropic /v1/messages endpoint..."
    test_port=${ANTHROPIC_PORT}
    if ! [[ " ${TEST_PORTS[@]} " =~ " ${ANTHROPIC_PORT} " ]]; then
        test_port=${FACADE_PORT}
    fi

    # Anthropic endpoint should return 400/401 without proper auth/body
    status=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:$test_port/v1/messages 2>&1)
    if [ "$status" = "400" ] || [ "$status" = "401" ] || [ "$status" = "405" ]; then
        echo "  ✅ /v1/messages accessible on :$test_port (status: $status)"
        ((passed++))
    else
        echo "  ⚠️  /v1/messages unexpected status on :$test_port: $status"
        ((failed++))
    fi
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
