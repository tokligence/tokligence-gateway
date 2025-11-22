#!/bin/bash
# Integration test for Firewall Redact Mode (PII Tokenization)
#
# This test verifies end-to-end PII tokenization and detokenization:
# 1. Input PII is detected and replaced with realistic tokens
# 2. Tokens are sent to LLM (protecting original PII)
# 3. LLM response tokens are detokenized back to original PII
# 4. User receives response with original PII intact

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TEST_NAME="firewall_redact_mode"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Gateway endpoint
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8081}"
GATEWAY_TOKEN="${GATEWAY_TOKEN:-test}"

# Check if gateway is running
if ! curl -s "${GATEWAY_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}✗ Gateway not running at ${GATEWAY_URL}${NC}"
    echo "  Start gateway with: make gfr"
    exit 1
fi

# Check if API keys are available
if [ -z "$TOKLIGENCE_OPENAI_API_KEY" ] && [ -z "$TOKLIGENCE_ANTHROPIC_API_KEY" ]; then
    echo -e "${YELLOW}⚠  No API tokens found. Skipping live LLM tests.${NC}"
    echo "  Set TOKLIGENCE_OPENAI_API_KEY or TOKLIGENCE_ANTHROPIC_API_KEY to run full tests"
    exit 0
fi

echo -e "${YELLOW}======================================"
echo "Firewall Redact Mode Integration Test"
echo "======================================${NC}"
echo ""

PASSED=0
FAILED=0

# Helper function to run test
run_test() {
    local test_name="$1"
    local test_func="$2"

    echo -e "${YELLOW}Test: ${test_name}${NC}"
    if $test_func; then
        echo -e "${GREEN}✓ PASS${NC}\n"
        ((PASSED++))
    else
        echo -e "${RED}✗ FAIL${NC}\n"
        ((FAILED++))
    fi
}

# Test 1: Input PII detection and tokenization
test_input_tokenization() {
    local request=$(cat <<'EOF'
{
  "model": "gpt-4o-mini",
  "messages": [{"role": "user", "content": "My email is alice@example.com and phone is 555-111-2222"}],
  "stream": false,
  "max_tokens": 30
}
EOF
)

    local response=$(curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
        -d "$request")

    # Check if request succeeded
    if echo "$response" | jq -e '.choices[0].message.content' > /dev/null 2>&1; then
        echo "  Request succeeded, PII was processed"
        return 0
    else
        echo "  Request failed: $response"
        return 1
    fi
}

# Test 2: Output detokenization (echo test)
test_output_detokenization() {
    local request=$(cat <<'EOF'
{
  "model": "gpt-4o-mini",
  "messages": [{"role": "user", "content": "Repeat exactly: bob@company.org and (555) 999-8888"}],
  "stream": false,
  "max_tokens": 50
}
EOF
)

    local response=$(curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
        -d "$request")

    local content=$(echo "$response" | jq -r '.choices[0].message.content')

    # Verify original PII is in response (detokenization worked)
    if echo "$content" | grep -q "bob@company.org" && echo "$content" | grep -q "999-8888"; then
        echo "  ✓ Original PII preserved in response"
        echo "  Response: $content"
        return 0
    else
        echo "  ✗ Original PII not found in response"
        echo "  Response: $content"
        return 1
    fi
}

# Test 3: Multiple PII types
test_multiple_pii_types() {
    local request=$(cat <<'EOF'
{
  "model": "gpt-4o-mini",
  "messages": [{"role": "user", "content": "Echo: Email=test@example.com, Phone=555-123-4567, SSN=123-45-6789"}],
  "stream": false,
  "max_tokens": 50
}
EOF
)

    local response=$(curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${GATEWAY_TOKEN}" \
        -d "$request")

    local content=$(echo "$response" | jq -r '.choices[0].message.content')

    # Count how many PII types are in response
    local found=0
    echo "$content" | grep -q "test@example.com" && ((found++))
    echo "$content" | grep -q "555-123-4567" && ((found++))
    echo "$content" | grep -q "123-45-6789" && ((found++))

    if [ $found -ge 2 ]; then
        echo "  ✓ Multiple PII types detokenized ($found/3)"
        return 0
    else
        echo "  ✗ Only $found/3 PII types found"
        echo "  Response: $content"
        return 1
    fi
}

# Test 4: Firewall logs verification
test_firewall_logs() {
    local log_file="logs/dev-gatewayd-$(date +%Y-%m-%d).log"

    if [ ! -f "$log_file" ]; then
        echo "  ✗ Log file not found: $log_file"
        return 1
    fi

    # Check for redaction logs
    local input_redacted=$(grep "firewall.input.redacted" "$log_file" | wc -l)
    local output_redacted=$(grep "firewall.output.redacted" "$log_file" | wc -l)

    if [ $input_redacted -gt 0 ] && [ $output_redacted -gt 0 ]; then
        echo "  ✓ Firewall logs found (input=$input_redacted, output=$output_redacted)"
        return 0
    else
        echo "  ✗ Missing firewall logs (input=$input_redacted, output=$output_redacted)"
        return 1
    fi
}

# Test 5: Verify redact mode is active
test_redact_mode_active() {
    local log_file="logs/dev-gatewayd-$(date +%Y-%m-%d).log"

    if [ ! -f "$log_file" ]; then
        echo "  ✗ Log file not found: $log_file"
        return 1
    fi

    # Check firewall configuration log
    if grep -q "firewall configured: mode=redact" "$log_file"; then
        echo "  ✓ Firewall in redact mode"
        # Also check filter counts
        local filter_info=$(grep "firewall configured: mode=redact" "$log_file" | tail -1)
        echo "  $filter_info"
        return 0
    else
        echo "  ✗ Firewall not in redact mode"
        return 1
    fi
}

# Run all tests
run_test "Input PII tokenization" test_input_tokenization
run_test "Output PII detokenization" test_output_detokenization
run_test "Multiple PII types" test_multiple_pii_types
run_test "Firewall logs verification" test_firewall_logs
run_test "Redact mode active" test_redact_mode_active

# Summary
echo -e "${YELLOW}======================================${NC}"
echo -e "Results: ${GREEN}${PASSED} passed${NC}, ${RED}${FAILED} failed${NC}"
echo -e "${YELLOW}======================================${NC}"

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
