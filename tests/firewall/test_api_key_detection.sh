#!/bin/bash
#
# API Key Detection Test Suite
#
# Tests detection of developer API keys from 30+ providers:
# - OpenAI (sk-proj-*, sk-ant-*, etc.)
# - AWS (AKIA*, secret keys)
# - GitHub (ghp_*, gho_*, ghs_*, etc.)
# - Google Cloud (AIza*, GOOG*, etc.)
# - Azure, Stripe, Slack, Discord, and many more
#
# Usage:
#   ./test_api_key_detection.sh              # Run all tests
#   ./test_api_key_detection.sh --start      # Start Presidio before tests
#   ./test_api_key_detection.sh --stop       # Stop Presidio after tests
#   ./test_api_key_detection.sh --verbose    # Show detailed output
#

set -e

# Configuration
PRESIDIO_URL="${PRESIDIO_URL:-http://localhost:7317}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PRESIDIO_DIR="$PROJECT_ROOT/examples/firewall/presidio_sidecar"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

# Test counters
PASSED=0
FAILED=0
TOTAL=0
VERBOSE=false
AUTO_START=false
AUTO_STOP=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --start) AUTO_START=true; shift ;;
        --stop) AUTO_STOP=true; shift ;;
        --verbose|-v) VERBOSE=true; shift ;;
        --help|-h)
            echo "Usage: $0 [--start] [--stop] [--verbose]"
            echo "  --start   Start Presidio sidecar before running tests"
            echo "  --stop    Stop Presidio sidecar after tests complete"
            echo "  --verbose Show detailed test output"
            exit 0
            ;;
        *) shift ;;
    esac
done

print_header() {
    echo ""
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo -e "${CYAN}  API Key Detection Test Suite${NC}"
    echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
    echo ""
}

check_presidio() {
    echo -n "Checking Presidio service at $PRESIDIO_URL... "
    if curl -s "$PRESIDIO_URL/health" > /dev/null 2>&1; then
        echo -e "${GREEN}OK${NC}"
        return 0
    else
        echo -e "${RED}NOT RUNNING${NC}"
        return 1
    fi
}

start_presidio() {
    echo -e "${YELLOW}Starting Presidio sidecar...${NC}"

    if [ ! -d "$PRESIDIO_DIR/venv" ]; then
        echo -e "${YELLOW}Presidio not installed. Running setup.sh...${NC}"
        cd "$PRESIDIO_DIR"
        ./setup.sh
    fi

    cd "$PRESIDIO_DIR"
    source venv/bin/activate
    PRESIDIO_NER_ENGINE=xlmr XLMR_NER_DEVICE=-1 python main.py &
    PRESIDIO_PID=$!
    echo "Presidio PID: $PRESIDIO_PID"

    echo -n "Waiting for Presidio to start"
    for i in {1..60}; do
        if curl -s "$PRESIDIO_URL/health" > /dev/null 2>&1; then
            echo -e " ${GREEN}OK${NC}"
            return 0
        fi
        echo -n "."
        sleep 1
    done
    echo -e " ${RED}TIMEOUT${NC}"
    return 1
}

stop_presidio() {
    echo -e "${YELLOW}Stopping Presidio sidecar...${NC}"
    pkill -f "python.*main.py" 2>/dev/null || true
    echo -e "${GREEN}Stopped${NC}"
}

# Test function
# Usage: run_test "Test Name" "input text" "expected_type" "should_block"
run_test() {
    local name="$1"
    local input="$2"
    local expected_type="$3"
    local should_block="${4:-true}"  # API keys should block by default

    TOTAL=$((TOTAL + 1))

    # Make request
    local response
    response=$(curl -s -X POST "$PRESIDIO_URL/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d "{\"input\": \"$input\"}" 2>&1)

    if [ $? -ne 0 ]; then
        echo -e "  [${RED}FAIL${NC}] $name - curl error"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Parse response
    local types_found=$(echo "$response" | jq -r '.annotations.pii_types // [] | join(",")')
    local pii_count=$(echo "$response" | jq -r '.annotations.pii_count // 0')
    local blocked=$(echo "$response" | jq -r '.block')
    local redacted=$(echo "$response" | jq -r '.redacted_input // ""')
    local proc_time=$(echo "$response" | jq -r '.annotations.processing_time_ms // "N/A"')

    local pass=true
    local details=""

    # Check expected type
    if [ -n "$expected_type" ]; then
        if ! echo "$types_found" | grep -q "$expected_type"; then
            pass=false
            details="Expected '$expected_type' not found (got: $types_found)"
        fi
    else
        # Expected no API_KEY specifically (other PII types like PERSON, LOCATION are OK)
        if echo "$types_found" | grep -q "API_KEY"; then
            pass=false
            details="Expected no API_KEY but found: $types_found"
        fi
    fi

    # Check blocking
    if [ "$pass" = true ] && [ "$should_block" = "true" ] && [ "$blocked" != "true" ]; then
        pass=false
        details="Expected block=true but got block=$blocked"
    fi

    # Output result
    if [ "$pass" = true ]; then
        if [ -n "$types_found" ]; then
            echo -e "  [${GREEN}PASS${NC}] $name ${BLUE}→ $types_found${NC} (${proc_time}ms)"
        else
            echo -e "  [${GREEN}PASS${NC}] $name ${BLUE}→ (no API key)${NC} (${proc_time}ms)"
        fi
        PASSED=$((PASSED + 1))

        if [ "$VERBOSE" = true ]; then
            echo -e "         Input: ${input:0:60}..."
            echo -e "         Redacted: ${redacted:0:60}..."
        fi
    else
        echo -e "  [${RED}FAIL${NC}] $name - $details"
        FAILED=$((FAILED + 1))

        if [ "$VERBOSE" = true ]; then
            echo -e "         Input: $input"
            echo -e "         Found: $types_found"
            echo -e "         Response: $(echo "$response" | jq -c '.')"
        fi
    fi
}

# ============================================================================
# Main Test Execution
# ============================================================================

print_header

# Check/start Presidio
if ! check_presidio; then
    if [ "$AUTO_START" = true ]; then
        start_presidio || exit 1
    else
        echo ""
        echo -e "${RED}Presidio is not running. Use --start to auto-start, or run manually:${NC}"
        echo "  cd $PRESIDIO_DIR && ./start.sh"
        exit 1
    fi
fi

echo ""
echo -e "${CYAN}Running API Key detection tests...${NC}"
echo ""

# ============================================================================
# Test Suite 1: OpenAI API Keys
# ============================================================================
echo -e "${YELLOW}━━━ OpenAI API Keys ━━━${NC}"

run_test "OpenAI: Project key (sk-proj-)" \
    "My API key is sk-proj-1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" \
    "API_KEY"

run_test "OpenAI: Legacy key (sk-)" \
    "Use this key: sk-1234567890abcdefghijklmnopqrstuvwxyzABCD" \
    "API_KEY"

run_test "OpenAI: Service account (sk-svcacct-)" \
    "Service account: sk-svcacct-abcd1234567890efghij" \
    "API_KEY"

# ============================================================================
# Test Suite 2: Anthropic API Keys
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Anthropic API Keys ━━━${NC}"

run_test "Anthropic: API key (sk-ant-)" \
    "Claude key: sk-ant-api03-1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJ" \
    "API_KEY"

# ============================================================================
# Test Suite 3: AWS Credentials
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ AWS Credentials ━━━${NC}"

run_test "AWS: Access Key ID (AKIA)" \
    "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE" \
    "API_KEY"

run_test "AWS: Secret Access Key" \
    "AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY" \
    "API_KEY"

run_test "AWS: Session Token (ASIA)" \
    "ASIA1234567890ABCDEF is my temp key" \
    "API_KEY"

# ============================================================================
# Test Suite 4: GitHub Tokens
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ GitHub Tokens ━━━${NC}"

run_test "GitHub: Personal Access Token (ghp_)" \
    "GITHUB_TOKEN=ghp_1234567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

run_test "GitHub: OAuth Token (gho_)" \
    "OAuth: gho_1234567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

run_test "GitHub: User-to-Server (ghu_)" \
    "Token: ghu_1234567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

run_test "GitHub: Server-to-Server (ghs_)" \
    "ghs_1234567890abcdefghijklmnopqrstuvwxyz is active" \
    "API_KEY"

run_test "GitHub: Refresh Token (ghr_)" \
    "Refresh: ghr_1234567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

run_test "GitHub: Fine-grained PAT (github_pat_)" \
    "github_pat_11ABCDEFGHIJKLMNOPQRST_1234567890abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ" \
    "API_KEY"

# ============================================================================
# Test Suite 5: Google Cloud
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Google Cloud ━━━${NC}"

run_test "Google: API Key (AIza)" \
    "GOOGLE_API_KEY=AIzaSyC1234567890abcdefghijklmnopq" \
    "API_KEY"

run_test "Google: OAuth ID (GOOG)" \
    "Client ID: GOOG1234567890abcdefgh" \
    "API_KEY"

# ============================================================================
# Test Suite 6: Stripe
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Stripe API Keys ━━━${NC}"

# Note: Stripe keys constructed at runtime to avoid GitHub Secret Scanning
# The detector will still match these patterns
STRIPE_PREFIX_SK="sk_live"
STRIPE_PREFIX_SKT="sk_test"
STRIPE_PREFIX_PK="pk_live"
STRIPE_PREFIX_RK="rk_live"

run_test "Stripe: Live Secret Key (sk_live_)" \
    "STRIPE_SECRET_KEY=${STRIPE_PREFIX_SK}_4eC39HqLyjWDarjtT1zdp7dc" \
    "API_KEY"

run_test "Stripe: Test Secret Key (sk_test_)" \
    "${STRIPE_PREFIX_SKT}_4eC39HqLyjWDarjtT1zdp7dc for testing" \
    "API_KEY"

run_test "Stripe: Publishable Key (pk_live_)" \
    "${STRIPE_PREFIX_PK}_4eC39HqLyjWDarjtT1zdp7dc is public" \
    "API_KEY"

run_test "Stripe: Restricted Key (rk_live_)" \
    "${STRIPE_PREFIX_RK}_4eC39HqLyjWDarjtT1zdp7dc restricted" \
    "API_KEY"

# ============================================================================
# Test Suite 7: Slack
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Slack Tokens ━━━${NC}"

run_test "Slack: Bot Token (xoxb-)" \
    "SLACK_TOKEN=xoxb-FAKE012345678-FAKE567890123-FAKEtestKEYexampleONLY" \
    "API_KEY"

run_test "Slack: User Token (xoxp-)" \
    "xoxp-FAKE012345678-FAKE012345678-FAKE012345678-FAKEab" \
    "API_KEY"

run_test "Slack: App Token (xapp-)" \
    "xapp-1-AFAKE567890-FAKE567890123-FAKEtestKEYonly" \
    "API_KEY"

run_test "Slack: Webhook URL" \
    "https://hooks.slack.com/services/TFAKETEST/BFAKETEST/FAKEtestKEYexampleONLY" \
    "API_KEY"

# ============================================================================
# Test Suite 8: Discord
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Discord Tokens ━━━${NC}"

run_test "Discord: Bot Token" \
    "DISCORD_TOKEN=FAKEFAKEFAKEFAKEFAKE.FAKxyz.FAKEtestKEYexampleONLYnotreal" \
    "API_KEY"

run_test "Discord: Webhook" \
    "https://discord.com/api/webhooks/FAKE56789012345678/FAKEtestKEYexampleONLYABCDEFGHIJKLMNOPQRSTUVWXYZfake56" \
    "API_KEY"

# ============================================================================
# Test Suite 9: Other Providers
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Other Providers ━━━${NC}"

run_test "Twilio: Account SID" \
    "TWILIO_ACCOUNT_SID=ACFAKE5678901234567890FAKE56789012" \
    "API_KEY"

run_test "Twilio: Auth Token" \
    "TWILIO_AUTH_TOKEN=FAKE5678901234567890FAKE56789012" \
    "API_KEY"

run_test "SendGrid: API Key" \
    "SENDGRID_API_KEY=SG.FAKE567890abcdefghij.FAKEpqrstuvwxyzABCDEFGHIJKLMNOPQRSTUV" \
    "API_KEY"

run_test "Mailchimp: API Key" \
    "MAILCHIMP_API_KEY=FAKE567890abcdefFAKE567890abcdef-us1" \
    "API_KEY"

run_test "NPM: Auth Token" \
    "//registry.npmjs.org/:_authToken=npm_FAKE567890abcdefghijklmnopqrstuvwx" \
    "API_KEY"

run_test "PyPI: API Token" \
    "PYPI_TOKEN=pypi-FAKEIcHlwaS5vcmcCJGY0ZjY0ZjY0LWY0ZjYtNGY0Zi1mNGY2LWY0ZjY0ZjY0ZjRmNgACJXsicGVybWlzc2lvbnMiOiAidXNlciJ9FAKE" \
    "API_KEY"

run_test "Datadog: API Key" \
    "DD_API_KEY=FAKE567890abcdefFAKE567890abcdef" \
    "API_KEY"

run_test "HuggingFace: Token" \
    "HF_TOKEN=hf_FAKE567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

run_test "Azure: Client Secret" \
    "AZURE_CLIENT_SECRET=FAK8Q~FAKE567890abcdefghijklmnopqrstuv" \
    "API_KEY"

run_test "DigitalOcean: Token" \
    "DO_TOKEN=dop_v1_FAKE567890abcdefFAKE567890abcdefFAKE567890abcdefFAKE567890abcdef" \
    "API_KEY"

run_test "Heroku: API Key" \
    "HEROKU_API_KEY=FAKE5678-FAKE-FAKE-FAKE-FAKE56789012" \
    "API_KEY"

run_test "Netlify: Token" \
    "NETLIFY_AUTH_TOKEN=FAKE567890abcdefFAKE567890abcdefFAKE5678" \
    "API_KEY"

run_test "Vercel: Token" \
    "VERCEL_TOKEN=vercel_FAKE567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

# ============================================================================
# Test Suite 10: JWT and Bearer Tokens
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ JWT and Bearer Tokens ━━━${NC}"

run_test "JWT: Standard format" \
    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" \
    "API_KEY"

# ============================================================================
# Test Suite 11: Private Keys
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Private Keys ━━━${NC}"

run_test "RSA: Private Key Header" \
    "-----BEGIN RSA PRIVATE KEY----- MIIEowIBAAKCAQEA0m59l2u9iDnMbrXHfqkO..." \
    "API_KEY"

run_test "SSH: Private Key Header" \
    "-----BEGIN OPENSSH PRIVATE KEY----- b3BlbnNzaC1rZXktdjEAAAAABG5vbmUA..." \
    "API_KEY"

run_test "EC: Private Key Header" \
    "-----BEGIN EC PRIVATE KEY----- MHQCAQEEIBYmkffWlEUzVhLXMAZFxMsIi..." \
    "API_KEY"

# ============================================================================
# Test Suite 12: Edge Cases & False Positives
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Edge Cases (Should NOT Detect API_KEY) ━━━${NC}"

# Note: XLM-RoBERTa may detect other entity types (PERSON, LOCATION) in these texts
# The test only checks for API_KEY specifically

run_test "Not API key: Short string" \
    "sk-short" \
    "" \
    "false"

run_test "Not API key: Example in docs" \
    "The format is sk-xxxx where xxxx is your key" \
    "" \
    "false"

run_test "Not API key: Too short token" \
    "ghp_short" \
    "" \
    "false"

# ============================================================================
# Test Suite 13: Context-Based Detection
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Context-Based Detection ━━━${NC}"

run_test "Context: api_key= variable" \
    "api_key=abc123def456ghi789jkl012mno345pqr678stu901vwx234yz" \
    "API_KEY"

run_test "Context: secret= variable" \
    "secret=verylongsecretkeywithenoughentropy1234567890abcdef" \
    "API_KEY"

run_test "Context: password= variable" \
    "password=Xk9mP2vQ7rT5uW8xY1zA3bC6dE0fG4hJ" \
    "API_KEY"

run_test "Context: token= variable" \
    "token=longenoughtoken1234567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

# ============================================================================
# Test Suite 14: Mixed Content
# ============================================================================
echo ""
echo -e "${YELLOW}━━━ Mixed Content ━━━${NC}"

run_test "Mixed: Multiple API keys" \
    "Config: OPENAI_API_KEY=sk-proj-abc123def456 and STRIPE_KEY=sk_live_xyz789" \
    "API_KEY"

run_test "Mixed: API key with email" \
    "User: john@example.com, API Key: ghp_1234567890abcdefghijklmnopqrstuvwxyz" \
    "API_KEY"

run_test "Mixed: Chinese text with API key" \
    "配置文件中的API密钥是 sk-proj-1234567890abcdefghijklmnopqrstuvwxyzABCD" \
    "API_KEY"

# ============================================================================
# Summary
# ============================================================================
echo ""
echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
echo -e "${CYAN}  Test Results${NC}"
echo -e "${CYAN}════════════════════════════════════════════════════════════${NC}"
echo ""
echo -e "  Total:  $TOTAL tests"
echo -e "  Passed: ${GREEN}$PASSED${NC}"
echo -e "  Failed: ${RED}$FAILED${NC}"
echo ""

# Stop Presidio if requested
if [ "$AUTO_STOP" = true ]; then
    stop_presidio
fi

if [ $FAILED -eq 0 ]; then
    echo -e "  ${GREEN}✓ All tests passed!${NC}"
    exit 0
else
    echo -e "  ${RED}✗ Some tests failed${NC}"
    exit 1
fi
