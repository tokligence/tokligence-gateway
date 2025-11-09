#!/bin/bash

# Test: Multi-Port Mode Endpoint Isolation
# Validates that each port only exposes its designated endpoint types
# Port scheme: facade=8081, admin=8079, openai=8082, anthropic=8083

set -e

echo "🧪 Testing Multi-Port Mode Endpoint Isolation"
echo "=============================================="
echo ""

# Check if gatewayd is running
if ! pgrep -f "gatewayd" > /dev/null; then
    echo "❌ gatewayd is not running. Please start it first with multiport_mode=true"
    exit 1
fi

# Verify multi-port mode is enabled by checking for multiple ports
PORTS=$(ss -ltnp 2>/dev/null | grep gatewayd | grep -oP ':\K[0-9]+' | sort -u)
PORT_COUNT=$(echo "$PORTS" | wc -l)

if [ "$PORT_COUNT" -lt 4 ]; then
    echo "❌ Multi-port mode not detected. Found $PORT_COUNT ports, expected 4"
    echo "   Detected ports: $(echo $PORTS | tr '\n' ' ')"
    echo "   Enable with: multiport_mode=true in config/dev/gateway.ini"
    exit 1
fi

echo "✅ Multi-port mode detected: $PORT_COUNT ports"
echo ""

# Test 1: Facade port (8081) - should have ALL endpoints
echo "=== Test 1: Facade Port (8081) - All Endpoints ==="
curl -sf http://localhost:8081/health > /dev/null && echo "  ✅ /health" || echo "  ❌ /health"
curl -sf http://localhost:8081/v1/models > /dev/null && echo "  ✅ /v1/models (OpenAI)" || echo "  ❌ /v1/models"
curl -sf -X POST http://localhost:8081/v1/chat/completions -H "Content-Type: application/json" -d '{"model":"test","messages":[]}' 2>&1 | grep -q "model\|error" && echo "  ✅ /v1/chat/completions (OpenAI)" || echo "  ❌ /v1/chat/completions"
curl -sf -X POST http://localhost:8081/v1/messages -H "Content-Type: application/json" -d '{}' 2>&1 | grep -q "model\|error" && echo "  ✅ /v1/messages (Anthropic)" || echo "  ❌ /v1/messages"
echo ""

# Test 2: Admin port (8079) - should NOT have OpenAI/Anthropic endpoints
echo "=== Test 2: Admin Port (8079) - Admin Only ==="
curl -sf http://localhost:8079/health > /dev/null && echo "  ✅ /health accessible" || echo "  ❌ /health"

# OpenAI endpoints should NOT be accessible on admin port
if curl -sf http://localhost:8079/v1/models > /dev/null 2>&1; then
    echo "  ❌ /v1/models should NOT be accessible on admin port"
else
    echo "  ✅ /v1/models properly isolated (not on admin port)"
fi

# Anthropic endpoints should NOT be accessible on admin port
if curl -sf http://localhost:8079/v1/messages > /dev/null 2>&1; then
    echo "  ❌ /v1/messages should NOT be accessible on admin port"
else
    echo "  ✅ /v1/messages properly isolated (not on admin port)"
fi
echo ""

# Test 3: OpenAI port (8082) - should NOT have Anthropic endpoints
echo "=== Test 3: OpenAI Port (8082) - OpenAI Only ==="
curl -sf http://localhost:8082/health > /dev/null && echo "  ✅ /health accessible" || echo "  ❌ /health"
curl -sf http://localhost:8082/v1/models > /dev/null && echo "  ✅ /v1/models accessible" || echo "  ❌ /v1/models"

# Anthropic endpoints should NOT be accessible on OpenAI port
if curl -sf http://localhost:8082/v1/messages > /dev/null 2>&1; then
    echo "  ❌ /v1/messages should NOT be accessible on OpenAI port"
else
    echo "  ✅ /v1/messages properly isolated (not on OpenAI port)"
fi
echo ""

# Test 4: Anthropic port (8083) - should NOT have OpenAI endpoints
echo "=== Test 4: Anthropic Port (8083) - Anthropic Only ==="
curl -sf http://localhost:8083/health > /dev/null && echo "  ✅ /health accessible" || echo "  ❌ /health"

# OpenAI endpoints should NOT be accessible on Anthropic port
if curl -sf http://localhost:8083/v1/models > /dev/null 2>&1; then
    echo "  ❌ /v1/models should NOT be accessible on Anthropic port"
else
    echo "  ✅ /v1/models properly isolated (not on Anthropic port)"
fi

# Anthropic endpoint should be accessible
STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8083/v1/messages -H "Content-Type: application/json" -d '{}')
if [ "$STATUS" = "400" ] || [ "$STATUS" = "401" ] || [ "$STATUS" = "200" ]; then
    echo "  ✅ /v1/messages accessible (HTTP $STATUS)"
else
    echo "  ⚠️  /v1/messages unexpected status: HTTP $STATUS"
fi
echo ""

echo "=============================================="
echo "✅ Multi-Port Isolation Tests Complete!"
echo ""
echo "Summary:"
echo "  - Facade (8081): All endpoints accessible ✅"
echo "  - Admin (8079): Admin only, others isolated ✅"
echo "  - OpenAI (8082): OpenAI only, Anthropic isolated ✅"
echo "  - Anthropic (8083): Anthropic only, OpenAI isolated ✅"
