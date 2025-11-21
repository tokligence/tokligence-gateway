#!/bin/bash
# Firewall PII Detection Integration Test
# Tests that the firewall correctly detects and logs PII in requests
#
# Requirements:
# - Gateway must be running (make gds)
# - config/firewall.yaml must be configured with input filters enabled
# - API tokens must be available in .env file (for real API tests)
#
# Usage:
#   ./test_pii_detection.sh [--skip-real-api]
#
# Exit codes:
#   0 - All tests passed
#   1 - One or more tests failed
#   2 - Configuration error (no gateway running, no firewall configured)

set -e

# Configuration
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8081}"
# Try to find the most recent log file
if [ -L "logs/dev-gatewayd.log" ]; then
    LOG_FILE="$(readlink -f logs/dev-gatewayd.log)"
elif [ -f "logs/dev-gatewayd-$(date +%Y-%m-%d).log" ]; then
    LOG_FILE="logs/dev-gatewayd-$(date +%Y-%m-%d).log"
else
    LOG_FILE=$(ls -t logs/dev-gatewayd-*.log 2>/dev/null | head -1)
fi
LOG_FILE="${LOG_FILE:-logs/dev-gatewayd.log}"
SKIP_REAL_API=false

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Parse arguments
for arg in "$@"; do
    case $arg in
        --skip-real-api)
            SKIP_REAL_API=true
            shift
            ;;
    esac
done

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((TESTS_PASSED++))
}

log_failure() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((TESTS_FAILED++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

run_test() {
    ((TESTS_RUN++))
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check if gateway is running
    if ! curl -s -f "${GATEWAY_URL}/health" > /dev/null 2>&1; then
        log_failure "Gateway is not running at ${GATEWAY_URL}"
        log_info "Start gateway with: make gds"
        exit 2
    fi
    log_success "Gateway is running"

    # Check if firewall config exists
    if [ ! -f "config/firewall.yaml" ]; then
        log_failure "Firewall config not found at config/firewall.yaml"
        exit 2
    fi
    log_success "Firewall config found"

    # Check if log file exists
    if [ ! -f "$LOG_FILE" ]; then
        log_warning "Log file not found at $LOG_FILE (may be created after first request)"
    else
        log_success "Log file found at $LOG_FILE"
    fi

    # Check if API keys are available
    if [ -f ".env" ]; then
        if grep -q "TOKLIGENCE_OPENAI_API_KEY" .env && grep -q "^TOKLIGENCE_OPENAI_API_KEY=sk-" .env; then
            log_success "OpenAI API key found in .env"
            export $(grep "^TOKLIGENCE_OPENAI_API_KEY" .env | xargs)
        else
            log_warning "OpenAI API key not found or invalid in .env (real API tests will be skipped)"
            SKIP_REAL_API=true
        fi
    else
        log_warning ".env file not found (real API tests will be skipped)"
        SKIP_REAL_API=true
    fi

    echo ""
}

# Test 1: Firewall initialization check
test_firewall_initialization() {
    run_test
    log_info "Test 1: Checking firewall initialization in logs..."

    if grep -q "firewall configured" "$LOG_FILE"; then
        local mode=$(grep "firewall configured" "$LOG_FILE" | tail -1 | grep -oP 'mode=\K\w+' || echo "unknown")
        local filters=$(grep "firewall configured" "$LOG_FILE" | tail -1 | grep -oP 'filters=\K\d+' || echo "0")

        if [ "$filters" -gt 0 ]; then
            log_success "Firewall initialized successfully (mode=$mode, filters=$filters)"
        else
            log_failure "Firewall initialized but no filters configured"
        fi
    else
        log_failure "Firewall not initialized in logs"
    fi
    echo ""
}

# Test 2: PII detection - Email and Phone
test_pii_detection_email_phone() {
    run_test
    log_info "Test 2: Testing PII detection (Email + Phone)..."

    # Clear recent logs or note timestamp
    local test_start=$(date +%s)
    sleep 1

    # Send request with PII
    local response=$(curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test" \
        -d '{
            "model": "loopback",
            "messages": [{
                "role": "user",
                "content": "My email is test@example.com and my phone is 555-1234."
            }],
            "stream": false
        }')

    # Check if request succeeded
    if echo "$response" | grep -q '"id"'; then
        log_success "Request processed successfully"
    else
        log_failure "Request failed: $response"
        echo ""
        return
    fi

    # Wait for log to be written
    sleep 2

    # Check logs for PII detection
    local recent_logs=$(tail -100 "$LOG_FILE")

    if echo "$recent_logs" | grep -q "firewall.detection.*EMAIL"; then
        log_success "Email PII detected in logs"
    else
        log_failure "Email PII not detected in logs"
    fi

    if echo "$recent_logs" | grep -q "firewall.detection.*PHONE"; then
        log_success "Phone PII detected in logs"
    else
        log_failure "Phone PII not detected in logs"
    fi

    if echo "$recent_logs" | grep -q "firewall.monitor.*pii_count=[2-9]"; then
        log_success "PII count recorded in monitor logs"
    else
        log_warning "PII count not found in monitor logs (check log level)"
    fi

    echo ""
}

# Test 3: Clean request (no PII)
test_clean_request() {
    run_test
    log_info "Test 3: Testing clean request (no PII)..."

    local response=$(curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test" \
        -d '{
            "model": "loopback",
            "messages": [{
                "role": "user",
                "content": "Hello, how are you today?"
            }],
            "stream": false
        }')

    if echo "$response" | grep -q '"id"'; then
        log_success "Clean request processed successfully"
    else
        log_failure "Clean request failed: $response"
    fi

    echo ""
}

# Test 4: Multiple PII types
test_multiple_pii_types() {
    run_test
    log_info "Test 4: Testing multiple PII types (Email, Phone, SSN)..."

    local response=$(curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test" \
        -d '{
            "model": "loopback",
            "messages": [{
                "role": "user",
                "content": "Contact: test@example.com, 555-123-4567, SSN: 123-45-6789"
            }],
            "stream": false
        }')

    if echo "$response" | grep -q '"id"'; then
        log_success "Request with multiple PII types processed"

        # Check logs
        sleep 2
        local recent_logs=$(tail -100 "$LOG_FILE")

        local pii_count=$(echo "$recent_logs" | grep "firewall.monitor" | tail -1 | grep -oP 'pii_count=\K\d+' || echo "0")
        if [ "$pii_count" -ge 3 ]; then
            log_success "Detected $pii_count PII entities (expected >= 3)"
        else
            log_failure "Expected >= 3 PII entities, detected $pii_count"
        fi
    else
        log_failure "Request failed: $response"
    fi

    echo ""
}

# Test 5: Real API test (if API keys available)
test_real_api() {
    if [ "$SKIP_REAL_API" = true ]; then
        log_warning "Skipping real API test (no API keys or --skip-real-api flag)"
        echo ""
        return
    fi

    run_test
    log_info "Test 5: Testing with real OpenAI API..."

    local response=$(curl -s -X POST "${GATEWAY_URL}/v1/chat/completions" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer test" \
        -d '{
            "model": "gpt-4o-mini",
            "messages": [{
                "role": "user",
                "content": "Say hello (do not include any personal information in your response)"
            }],
            "stream": false,
            "max_tokens": 50
        }')

    if echo "$response" | grep -q '"choices"'; then
        log_success "Real API request processed successfully"

        # Verify no PII in response
        sleep 2
        local recent_logs=$(tail -50 "$LOG_FILE")
        if echo "$recent_logs" | grep -q "firewall.monitor.*location=output"; then
            log_info "Output firewall check performed"
        fi
    else
        local error=$(echo "$response" | grep -oP '"error".*"message":\s*"\K[^"]+' || echo "Unknown error")
        log_failure "Real API request failed: $error"
    fi

    echo ""
}

# Main execution
main() {
    echo "============================================"
    echo "Firewall PII Detection Integration Test"
    echo "============================================"
    echo ""

    check_prerequisites
    test_firewall_initialization
    test_pii_detection_email_phone
    test_clean_request
    test_multiple_pii_types
    test_real_api

    # Summary
    echo "============================================"
    echo "Test Summary"
    echo "============================================"
    echo "Tests run:    $TESTS_RUN"
    echo "Tests passed: $TESTS_PASSED"
    echo "Tests failed: $TESTS_FAILED"
    echo ""

    if [ $TESTS_FAILED -eq 0 ]; then
        log_success "All tests passed!"
        exit 0
    else
        log_failure "$TESTS_FAILED test(s) failed"
        exit 1
    fi
}

main
