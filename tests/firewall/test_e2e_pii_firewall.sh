#!/bin/bash
#
# End-to-End PII Firewall Test
#
# Tests the complete flow: Client → Gateway (with Firewall) → LLM Provider
# Requires: Gateway running with Presidio sidecar enabled
#
# Usage:
#   ./test_e2e_pii_firewall.sh              # Run with existing services
#   ./test_e2e_pii_firewall.sh --setup      # Setup and start all services first
#   ./test_e2e_pii_firewall.sh --cleanup    # Stop services after tests
#

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8081}"
PRESIDIO_URL="${PRESIDIO_URL:-http://localhost:7317}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0
SETUP=false
CLEANUP=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --setup) SETUP=true; shift ;;
        --cleanup) CLEANUP=true; shift ;;
        *) shift ;;
    esac
done

print_header() {
    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}  End-to-End PII Firewall Test${NC}"
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo ""
}

check_services() {
    local all_ok=true

    echo -n "Gateway ($GATEWAY_URL): "
    if curl -s "$GATEWAY_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}OK${NC}"
    else
        echo -e "${RED}NOT RUNNING${NC}"
        all_ok=false
    fi

    echo -n "Presidio ($PRESIDIO_URL): "
    if curl -s "$PRESIDIO_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}OK${NC}"
    else
        echo -e "${RED}NOT RUNNING${NC}"
        all_ok=false
    fi

    if [ "$all_ok" = false ]; then
        return 1
    fi
    return 0
}

setup_services() {
    echo -e "${YELLOW}Setting up services...${NC}"
    cd "$PROJECT_ROOT"

    # Start Presidio
    echo "Starting Presidio sidecar..."
    make pii-start || true
    sleep 3

    # Create temp firewall config with Presidio enabled
    cat > /tmp/firewall-e2e.ini << 'EOF'
[prompt_firewall]
enabled = true
mode = monitor

pii_patterns_file = config/pii_patterns.ini
pii_regions = global,us,cn
pii_min_confidence = 0.70
log_decisions = true

[tokenizer]
store_type = memory
ttl = 1h

[firewall_input_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10

filter_presidio_enabled = true
filter_presidio_priority = 20
filter_presidio_endpoint = http://localhost:7317/v1/filter/input
filter_presidio_timeout_ms = 1000
filter_presidio_on_error = allow

[firewall_output_filters]
filter_pii_regex_enabled = true
filter_pii_regex_priority = 10
EOF

    # Start Gateway with firewall enabled
    echo "Starting Gateway with firewall..."
    export TOKLIGENCE_PROMPT_FIREWALL_CONFIG=/tmp/firewall-e2e.ini
    make gfr || true
    sleep 3
}

cleanup_services() {
    echo -e "${YELLOW}Cleaning up services...${NC}"
    cd "$PROJECT_ROOT"
    make pii-stop || true
    make gdx || true
}

run_test() {
    local name="$1"
    local endpoint="$2"
    local payload="$3"
    local check_field="$4"
    local expected="$5"

    TOTAL=$((TOTAL + 1))
    echo -n "  [$TOTAL] $name... "

    local response
    response=$(curl -s -X POST "$GATEWAY_URL$endpoint" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test-key" \
        -d "$payload" 2>&1)

    if [ $? -ne 0 ]; then
        echo -e "${RED}FAIL${NC} (curl error)"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Check for error
    local error=$(echo "$response" | jq -r '.error // empty')
    if [ -n "$error" ]; then
        echo -e "${RED}FAIL${NC} (error: $error)"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Check expected field
    if [ -n "$check_field" ] && [ -n "$expected" ]; then
        local actual=$(echo "$response" | jq -r "$check_field // empty")
        if echo "$actual" | grep -q "$expected"; then
            echo -e "${GREEN}PASS${NC}"
            PASSED=$((PASSED + 1))
        else
            echo -e "${RED}FAIL${NC} (expected '$expected' in $check_field)"
            FAILED=$((FAILED + 1))
        fi
    else
        # Just check request succeeded
        echo -e "${GREEN}PASS${NC}"
        PASSED=$((PASSED + 1))
    fi
}

# Test via OpenAI Chat Completions API
test_openai_chat() {
    echo -e "\n${YELLOW}━━━ OpenAI Chat Completions API ━━━${NC}"

    run_test "Basic request (no PII)" \
        "/v1/chat/completions" \
        '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"What is 2+2?"}],"max_tokens":50}' \
        ".choices[0].message.content" \
        ""

    run_test "Request with email" \
        "/v1/chat/completions" \
        '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"My email is test@example.com, say hi"}],"max_tokens":50}' \
        ".choices[0].message" \
        ""

    run_test "Request with Chinese name" \
        "/v1/chat/completions" \
        '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"我叫张三，请问今天天气如何？"}],"max_tokens":50}' \
        ".choices[0].message" \
        ""
}

# Test via Anthropic Messages API
test_anthropic_messages() {
    echo -e "\n${YELLOW}━━━ Anthropic Messages API ━━━${NC}"

    run_test "Basic request (no PII)" \
        "/anthropic/v1/messages" \
        '{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"What is 2+2?"}],"max_tokens":50}' \
        ".content[0].text" \
        ""

    run_test "Request with phone number" \
        "/anthropic/v1/messages" \
        '{"model":"claude-3-5-sonnet-20241022","messages":[{"role":"user","content":"Call me at 13800138000"}],"max_tokens":50}' \
        ".content[0]" \
        ""
}

# Test Presidio directly (sanity check)
test_presidio_direct() {
    echo -e "\n${YELLOW}━━━ Presidio Direct Tests ━━━${NC}"

    echo -n "  [D1] Chinese name detection... "
    local resp=$(curl -s -X POST "$PRESIDIO_URL/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d '{"input":"我叫张三"}')
    if echo "$resp" | jq -e '.annotations.pii_types | index("PERSON")' > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
    fi

    echo -n "  [D2] Email detection... "
    resp=$(curl -s -X POST "$PRESIDIO_URL/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d '{"input":"Email: john@test.com"}')
    if echo "$resp" | jq -e '.annotations.pii_types | index("EMAIL_ADDRESS")' > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
    fi

    echo -n "  [D3] Chinese ID card detection... "
    resp=$(curl -s -X POST "$PRESIDIO_URL/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d '{"input":"身份证：110101199001011234"}')
    if echo "$resp" | jq -e '.annotations.pii_types | index("CN_ID_CARD")' > /dev/null 2>&1; then
        echo -e "${GREEN}PASS${NC}"
    else
        echo -e "${RED}FAIL${NC}"
    fi
}

# ============================================================================
# Main
# ============================================================================

print_header

if [ "$SETUP" = true ]; then
    setup_services
fi

echo "Checking services..."
if ! check_services; then
    echo ""
    echo -e "${RED}Services not running. Use --setup to start them.${NC}"
    echo "Or manually run:"
    echo "  make pii-start   # Start Presidio"
    echo "  make gfr         # Start Gateway"
    exit 1
fi

# Run tests
test_presidio_direct
test_openai_chat
test_anthropic_messages

# Summary
echo ""
echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  Results${NC}"
echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
echo ""
echo "  Total:  $TOTAL"
echo -e "  Passed: ${GREEN}$PASSED${NC}"
echo -e "  Failed: ${RED}$FAILED${NC}"
echo ""

if [ "$CLEANUP" = true ]; then
    cleanup_services
fi

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
