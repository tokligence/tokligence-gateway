#!/bin/bash
# Firewall Integration Test Script
# 这个脚本帮助你验证 firewall 功能是否正常工作

set -e

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8081}"
PRESIDIO_URL="${PRESIDIO_URL:-http://localhost:7317}"

echo "=== Tokligence Gateway Firewall Test ==="
echo "Gateway URL: $GATEWAY_URL"
echo "Presidio URL: $PRESIDIO_URL"
echo ""

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Helper functions
pass() {
    echo -e "${GREEN}✓ PASS:${NC} $1"
    ((TESTS_PASSED++))
}

fail() {
    echo -e "${RED}✗ FAIL:${NC} $1"
    ((TESTS_FAILED++))
}

warn() {
    echo -e "${YELLOW}⚠ WARN:${NC} $1"
}

# Test 1: Check if gateway is running
echo "Test 1: Gateway health check"
if curl -s -f "$GATEWAY_URL/health" > /dev/null 2>&1; then
    pass "Gateway is running"
else
    fail "Gateway is not running at $GATEWAY_URL"
    echo "  Please start gateway first: make gds"
    exit 1
fi
echo ""

# Test 2: Check firewall logs
echo "Test 2: Check firewall initialization"
if [ -f "logs/gatewayd.log" ]; then
    if grep -q "firewall configured" logs/gatewayd.log; then
        MODE=$(grep "firewall configured" logs/gatewayd.log | tail -1 | grep -oP 'mode=\K\w+' || echo "unknown")
        FILTERS=$(grep "firewall configured" logs/gatewayd.log | tail -1 | grep -oP 'filters=\K\d+' || echo "0")
        pass "Firewall configured (mode=$MODE, filters=$FILTERS)"
    else
        warn "Firewall not found in logs (may not be enabled)"
    fi
else
    warn "Log file not found at logs/gatewayd.log"
fi
echo ""

# Test 3: Test with clean request
echo "Test 3: Clean request (no PII)"
RESPONSE=$(curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test" \
    -d '{
        "model": "loopback",
        "messages": [{"role": "user", "content": "Hello, how are you?"}]
    }' 2>&1 || true)

if echo "$RESPONSE" | grep -q '"content"'; then
    pass "Clean request processed successfully"
elif echo "$RESPONSE" | grep -q "Connection refused"; then
    fail "Cannot connect to gateway"
else
    warn "Unexpected response: $RESPONSE"
fi
echo ""

# Test 4: Test with PII
echo "Test 4: Request with PII (email)"
RESPONSE=$(curl -s -X POST "$GATEWAY_URL/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test" \
    -d '{
        "model": "loopback",
        "messages": [{"role": "user", "content": "My email is test@example.com"}]
    }' 2>&1 || true)

# Check logs for PII detection
sleep 1
if [ -f "logs/gatewayd.log" ]; then
    if grep -q "firewall.monitor.*EMAIL" logs/gatewayd.log | tail -20; then
        pass "PII detected in logs"
    else
        warn "PII not detected in logs (check firewall configuration)"
    fi
else
    warn "Cannot check logs"
fi
echo ""

# Test 5: Check if Presidio is running (optional)
echo "Test 5: Presidio health check (optional)"
if curl -s -f "$PRESIDIO_URL/health" > /dev/null 2>&1; then
    HEALTH=$(curl -s "$PRESIDIO_URL/health")
    if echo "$HEALTH" | grep -q '"status": "healthy"'; then
        pass "Presidio is running and healthy"

        # Test Presidio API
        echo "Test 5a: Presidio PII detection"
        PRESIDIO_RESPONSE=$(curl -s -X POST "$PRESIDIO_URL/v1/filter/input" \
            -H "Content-Type: application/json" \
            -d '{"input": "My SSN is 123-45-6789"}')

        if echo "$PRESIDIO_RESPONSE" | grep -q '"block": true'; then
            pass "Presidio correctly detected critical PII"
        elif echo "$PRESIDIO_RESPONSE" | grep -q '"pii_count"'; then
            pass "Presidio detected PII"
        else
            warn "Unexpected Presidio response"
        fi
    else
        warn "Presidio returned unhealthy status"
    fi
else
    warn "Presidio is not running (this is optional)"
    echo "  To start Presidio: cd examples/firewall/presidio_sidecar && ./start.sh"
fi
echo ""

# Test 6: Check configuration
echo "Test 6: Configuration check"
if [ -f "config/firewall.yaml" ]; then
    pass "Firewall configuration exists"
    MODE=$(grep "^mode:" config/firewall.yaml | awk '{print $2}' || echo "not found")
    ENABLED=$(grep "^enabled:" config/firewall.yaml | awk '{print $2}' || echo "not found")
    echo "  Mode: $MODE"
    echo "  Enabled: $ENABLED"
else
    warn "No firewall configuration found"
    echo "  Copy example: cp examples/firewall/configs/firewall.yaml config/"
fi
echo ""

# Summary
echo "=== Test Summary ==="
echo "Tests passed: $TESTS_PASSED"
echo "Tests failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Check logs: tail -f logs/gatewayd.log | grep firewall"
    echo "2. Send real requests with your API key"
    echo "3. Tune configuration in config/firewall.yaml"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo ""
    echo "Troubleshooting:"
    echo "1. Check gateway logs: tail -f logs/gatewayd.log"
    echo "2. Verify configuration: cat config/firewall.yaml"
    echo "3. See deployment guide: examples/firewall/DEPLOYMENT_GUIDE.md"
    exit 1
fi
