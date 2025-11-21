#!/bin/bash
# Basic Firewall Test - Simple demonstration of PII detection
#
# This is a minimal test that demonstrates the firewall is working.
# For comprehensive tests, see test_pii_detection.sh
#
# Requirements:
# - Gateway running (make gds)
# - Firewall enabled in config/firewall.yaml
#
# Usage: ./test_firewall_basic.sh

set -euo pipefail

GATEWAY_URL="${GATEWAY_URL:-http://localhost:8081}"
LOG_FILE=$(ls -t logs/dev-gatewayd-*.log 2>/dev/null | head -1 || echo "logs/dev-gatewayd.log")

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo "=========================================="
echo "Firewall Basic Test"
echo "=========================================="
echo ""

# Check gateway
echo -n "Checking gateway... "
if curl -sf "${GATEWAY_URL}/health" > /dev/null; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}✗${NC} Gateway not running at ${GATEWAY_URL}"
    echo "Start with: make gds"
    exit 1
fi

# Check firewall config
echo -n "Checking firewall config... "
if [ -f "config/firewall.yaml" ] && grep -q "enabled: true" config/firewall.yaml; then
    echo -e "${GREEN}✓${NC}"
else
    echo -e "${YELLOW}✗${NC} Firewall not enabled in config/firewall.yaml"
    exit 1
fi

echo ""
echo "Test 1: Sending request with PII (email + phone)..."
echo "----------------------------------------------"

# Send request with PII
curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer test" \
    -d '{
        "model": "loopback",
        "messages": [{
            "role": "user",
            "content": "Contact info: john.doe@example.com, phone: 555-123-4567"
        }],
        "stream": false
    }' > /dev/null

echo -e "${BLUE}Request sent${NC}"
sleep 1

echo ""
echo "Checking logs for PII detections..."
echo "----------------------------------------------"

# Check for firewall logs
if [ -f "$LOG_FILE" ]; then
    echo "Looking in: $LOG_FILE"
    echo ""

    # Show firewall detections from last 50 lines
    tail -50 "$LOG_FILE" | grep "firewall" | tail -10 || echo "No firewall logs in recent entries"
else
    echo "Log file not found: $LOG_FILE"
fi

echo ""
echo "=========================================="
echo "Test complete!"
echo "=========================================="
echo ""
echo "Expected output:"
echo "  - firewall.detection ... EMAIL"
echo "  - firewall.detection ... PHONE"
echo "  - firewall.monitor location=input pii_count=2"
echo ""
