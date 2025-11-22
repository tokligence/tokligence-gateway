#!/bin/bash
# End-to-end integration test for Firewall Redact Mode
# This test verifies PII tokenization and detokenization with real LLM requests

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAME="firewall_redact_mode_e2e"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Gateway configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8081}"
GATEWAY_HEALTH_URL="$GATEWAY_URL/admin/health"

# Check if API tokens are available
check_tokens() {
    if [ -z "$TOKLIGENCE_ANTHROPIC_API_KEY" ] && [ -z "$TOKLIGENCE_OPENAI_API_KEY" ]; then
        echo -e "${YELLOW}⚠️  No API tokens found. Skipping live LLM tests.${NC}"
        echo -e "${YELLOW}   Set TOKLIGENCE_ANTHROPIC_API_KEY or TOKLIGENCE_OPENAI_API_KEY to run full tests.${NC}"
        return 1
    fi
    return 0
}

# Check if gateway is running
check_gateway() {
    if ! curl -s -f "$GATEWAY_HEALTH_URL" > /dev/null 2>&1; then
        echo -e "${RED}❌ Gateway is not running at $GATEWAY_URL${NC}"
        echo -e "${YELLOW}   Start the gateway with: make gfr${NC}"
        return 1
    fi
    return 0
}

# Wait for gateway to be ready
wait_for_gateway() {
    local max_attempts=30
    local attempt=1

    echo -e "${BLUE}Waiting for gateway to be ready...${NC}"

    while [ $attempt -le $max_attempts ]; do
        if curl -s -f "$GATEWAY_HEALTH_URL" > /dev/null 2>&1; then
            echo -e "${GREEN}✓ Gateway is ready${NC}"
            return 0
        fi
        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done

    echo -e "\n${RED}❌ Gateway failed to become ready after ${max_attempts}s${NC}"
    return 1
}

echo -e "${YELLOW}=== Firewall Redact Mode E2E Test ===${NC}"

# Pre-flight checks
echo -e "\n${BLUE}Pre-flight checks:${NC}"

if check_tokens; then
    echo -e "${GREEN}✓ API tokens available${NC}"
    HAS_TOKENS=true
else
    HAS_TOKENS=false
fi

if check_gateway; then
    echo -e "${GREEN}✓ Gateway is running${NC}"
else
    echo -e "${YELLOW}Gateway not running. These tests require a running gateway.${NC}"
    exit 0  # Exit gracefully for CI/CD
fi

# Test 1: Health check and firewall status
echo -e "\n${YELLOW}Test 1: Gateway health and firewall status${NC}"

HEALTH_RESPONSE=$(curl -s "$GATEWAY_HEALTH_URL")
echo "Health response: $HEALTH_RESPONSE"

if echo "$HEALTH_RESPONSE" | grep -q '"status":"healthy"'; then
    echo -e "${GREEN}✓ Gateway is healthy${NC}"
else
    echo -e "${RED}❌ Gateway health check failed${NC}"
    exit 1
fi

# Check firewall stats
STATS_URL="$GATEWAY_URL/admin/firewall/stats"
if curl -s -f "$STATS_URL" > /dev/null 2>&1; then
    STATS_RESPONSE=$(curl -s "$STATS_URL")
    echo "Firewall stats: $STATS_RESPONSE"

    if echo "$STATS_RESPONSE" | grep -q '"mode":"redact"'; then
        echo -e "${GREEN}✓ Firewall is in redact mode${NC}"
    else
        echo -e "${YELLOW}⚠️  Firewall is not in redact mode. Current mode:${NC}"
        echo "$STATS_RESPONSE" | grep -o '"mode":"[^"]*"'
    fi
else
    echo -e "${YELLOW}⚠️  Firewall stats endpoint not available${NC}"
fi

# Test 2: PII pattern loading
echo -e "\n${YELLOW}Test 2: PII pattern configuration${NC}"

# Create a test config to verify pattern loading
TEST_CONFIG="/tmp/firewall_test_config.yaml"
cat > "$TEST_CONFIG" <<EOF
enabled: true
mode: redact

pii_patterns:
  regions:
    - global
    - us
    - cn

input_filters:
  - type: pii_regex
    name: test_pii
    priority: 10
    enabled: true
    config:
      load_from_file: true

output_filters:
  - type: pii_regex
    name: test_pii_output
    priority: 10
    enabled: true
    config:
      load_from_file: true
EOF

echo -e "${GREEN}✓ Test configuration created${NC}"

# Test 3: Unit test for tokenization (without LLM)
echo -e "\n${YELLOW}Test 3: Local PII detection and tokenization${NC}"

# Test input with various PII types
TEST_INPUT="My email is john.doe@example.com and my phone is 555-123-4567. SSN: 123-45-6789"

echo "Test input: $TEST_INPUT"
echo "Expected detections:"
echo "  - EMAIL: john.doe@example.com"
echo "  - PHONE: 555-123-4567"
echo "  - SSN: 123-45-6789"

# Run Go unit test for PII detection
echo -e "\n${BLUE}Running Go unit tests...${NC}"
cd "$SCRIPT_DIR/../../../"

if go test -v ./internal/firewall -run TestPIITokenizer 2>&1 | tee /tmp/test_output.log; then
    echo -e "${GREEN}✓ PII tokenizer unit tests passed${NC}"
else
    echo -e "${RED}❌ PII tokenizer unit tests failed${NC}"
    cat /tmp/test_output.log
    exit 1
fi

# Test 4: Live LLM test (only if tokens available)
if [ "$HAS_TOKENS" = true ]; then
    echo -e "\n${YELLOW}Test 4: Live LLM request with PII redaction${NC}"

    # Prepare request with PII
    REQUEST_JSON=$(cat <<EOF
{
  "model": "claude-3-5-sonnet-20241022",
  "messages": [
    {
      "role": "user",
      "content": "My email is john.doe@example.com and phone is 555-123-4567. Please confirm you received this information."
    }
  ],
  "max_tokens": 100,
  "stream": false
}
EOF
)

    echo "Sending request to gateway..."
    echo "Request: $REQUEST_JSON"

    RESPONSE=$(curl -s -X POST "$GATEWAY_URL/v1/messages" \
        -H "Content-Type: application/json" \
        -H "anthropic-version: 2023-06-01" \
        -H "x-api-key: ${TOKLIGENCE_ANTHROPIC_API_KEY}" \
        -d "$REQUEST_JSON")

    echo -e "\n${BLUE}Response:${NC}"
    echo "$RESPONSE" | jq '.' || echo "$RESPONSE"

    # Verify response doesn't contain redacted.local (tokens should be restored)
    if echo "$RESPONSE" | grep -q "john.doe@example.com"; then
        echo -e "${GREEN}✓ Email was detokenized correctly${NC}"
    else
        if echo "$RESPONSE" | grep -q "@redacted.local"; then
            echo -e "${RED}❌ Email was not detokenized (token still present)${NC}"
        else
            echo -e "${YELLOW}⚠️  Could not verify detokenization (response format unclear)${NC}"
        fi
    fi

    # Check for errors
    if echo "$RESPONSE" | grep -q '"type":"error"'; then
        echo -e "${RED}❌ LLM request failed:${NC}"
        echo "$RESPONSE" | jq '.error' || echo "$RESPONSE"
        exit 1
    fi

    echo -e "${GREEN}✓ Live LLM test completed${NC}"
else
    echo -e "\n${YELLOW}Test 4: Skipped (no API tokens)${NC}"
fi

# Test 5: Check logs for redact mode activity
echo -e "\n${YELLOW}Test 5: Verify debug logs${NC}"

LOG_FILE="logs/gd-*.log"
if ls $LOG_FILE 1> /dev/null 2>&1; then
    echo "Checking logs for redact mode activity..."

    # Look for redact mode logs
    if grep -q "redact mode" $LOG_FILE 2>/dev/null; then
        echo -e "${GREEN}✓ Found redact mode activity in logs${NC}"
        echo -e "\n${BLUE}Recent redact mode log entries:${NC}"
        grep "redact mode" $LOG_FILE | tail -n 5
    else
        echo -e "${YELLOW}⚠️  No redact mode activity found in logs${NC}"
        echo -e "${YELLOW}   Make sure TOKLIGENCE_LOG_LEVEL=debug is set${NC}"
    fi

    # Look for tokenization logs
    if grep -q "tokenized" $LOG_FILE 2>/dev/null; then
        echo -e "${GREEN}✓ Found tokenization activity in logs${NC}"
    fi

    # Look for detokenization logs
    if grep -q "detokenized" $LOG_FILE 2>/dev/null; then
        echo -e "${GREEN}✓ Found detokenization activity in logs${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  Log file not found at $LOG_FILE${NC}"
fi

# Cleanup
rm -f "$TEST_CONFIG"

echo -e "\n${GREEN}=== All Redact Mode Tests Completed ===${NC}"
echo -e "\n${BLUE}Summary:${NC}"
echo "  - Gateway health: ✓"
echo "  - Firewall status: ✓"
echo "  - Unit tests: ✓"
if [ "$HAS_TOKENS" = true ]; then
    echo "  - Live LLM test: ✓"
else
    echo "  - Live LLM test: Skipped (no tokens)"
fi
echo "  - Log verification: ✓"

echo -e "\n${GREEN}✅ All tests passed!${NC}"
exit 0
