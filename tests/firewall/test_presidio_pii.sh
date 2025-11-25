#!/bin/bash
# Test script for Presidio PII Detection
# Tests person name detection and other PII types via Microsoft Presidio sidecar

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PRESIDIO_URL="${PRESIDIO_URL:-http://localhost:7317}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PRESIDIO_DIR="$PROJECT_ROOT/examples/firewall/presidio_sidecar"

# Test counters
PASSED=0
FAILED=0
TOTAL=0

print_header() {
    echo ""
    echo "================================================"
    echo "  Presidio PII Detection Tests"
    echo "================================================"
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
    echo "Starting Presidio sidecar..."
    if [ ! -d "$PRESIDIO_DIR/venv" ]; then
        echo -e "${YELLOW}Presidio not installed. Running setup.sh...${NC}"
        cd "$PRESIDIO_DIR"
        ./setup.sh
    fi

    cd "$PRESIDIO_DIR"
    ./start.sh &

    # Wait for startup
    echo -n "Waiting for Presidio to start"
    for i in {1..30}; do
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

run_test() {
    local name="$1"
    local input="$2"
    local expected_types="$3"
    local expected_redacted="$4"
    local should_block="${5:-false}"

    TOTAL=$((TOTAL + 1))
    echo -n "  [$TOTAL] $name... "

    local response
    response=$(curl -s -X POST "$PRESIDIO_URL/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d "{\"input\": \"$input\"}" 2>&1)

    if [ $? -ne 0 ]; then
        echo -e "${RED}FAILED${NC} (curl error)"
        FAILED=$((FAILED + 1))
        return 1
    fi

    # Check if expected types are detected
    local types_found=$(echo "$response" | jq -r '.annotations.pii_types // [] | join(",")')
    local redacted=$(echo "$response" | jq -r '.redacted_input // ""')
    local blocked=$(echo "$response" | jq -r '.block')

    local pass=true
    local details=""

    # Check expected types
    for expected_type in $(echo "$expected_types" | tr ',' ' '); do
        if ! echo "$types_found" | grep -q "$expected_type"; then
            pass=false
            details="Expected type '$expected_type' not found. Got: $types_found"
            break
        fi
    done

    # Check redaction if specified
    if [ "$pass" = true ] && [ -n "$expected_redacted" ]; then
        if [ "$redacted" != "$expected_redacted" ]; then
            pass=false
            details="Redaction mismatch. Expected: '$expected_redacted', Got: '$redacted'"
        fi
    fi

    # Check blocking
    if [ "$pass" = true ] && [ "$should_block" = "true" ] && [ "$blocked" != "true" ]; then
        pass=false
        details="Expected request to be blocked but it was allowed"
    fi

    if [ "$pass" = true ]; then
        echo -e "${GREEN}PASSED${NC}"
        PASSED=$((PASSED + 1))
    else
        echo -e "${RED}FAILED${NC}"
        echo "      $details"
        echo "      Response: $(echo "$response" | jq -c '.')"
        FAILED=$((FAILED + 1))
    fi
}

print_header

# Check if Presidio is running, start if needed
if ! check_presidio; then
    echo ""
    start_presidio || exit 1
    echo ""
fi

echo "Running PII detection tests..."
echo ""

# Test 1: English person names
echo "=== Person Name Detection (English) ==="
run_test "Single English name" \
    "My name is John Smith" \
    "PERSON" \
    "My name is [PERSON]"

run_test "Multiple English names" \
    "John Smith and Lisa Chen work together" \
    "PERSON"

run_test "Full name with title" \
    "Please contact Dr. Michael Johnson at the office" \
    "PERSON"

# Test 2: Email detection
echo ""
echo "=== Email Detection ==="
run_test "Simple email" \
    "Contact me at john.doe@example.com" \
    "EMAIL_ADDRESS" \
    "Contact me at [EMAIL]"

run_test "Email with person name" \
    "John Smith's email is john@company.com" \
    "PERSON,EMAIL_ADDRESS"

# Test 3: Phone numbers
echo ""
echo "=== Phone Number Detection ==="
run_test "US phone number" \
    "Call me at 555-123-4567" \
    "PHONE_NUMBER"

run_test "International phone" \
    "My number is +1-555-123-4567" \
    "PHONE_NUMBER"

# Test 4: Critical PII (should block)
echo ""
echo "=== Critical PII Detection (SSN, Credit Card) ==="
run_test "US SSN" \
    "My SSN is 123-45-6789" \
    "US_SSN" \
    "" \
    "true"

run_test "Credit card number" \
    "Pay with card 4111-1111-1111-1111" \
    "CREDIT_CARD" \
    "" \
    "true"

run_test "Multiple critical PII" \
    "Customer SSN: 123-45-6789, Card: 4111111111111111" \
    "US_SSN,CREDIT_CARD" \
    "" \
    "true"

# Test 5: Mixed PII
echo ""
echo "=== Mixed PII Detection ==="
run_test "Name + Email + Phone" \
    "John Smith, john@example.com, 555-123-4567" \
    "PERSON,EMAIL_ADDRESS,PHONE_NUMBER"

run_test "Full customer info" \
    "Customer: Michael Johnson, Email: mj@company.com, SSN: 123-45-6789" \
    "PERSON,EMAIL_ADDRESS,US_SSN" \
    "" \
    "true"

# Test 6: Location detection
echo ""
echo "=== Location Detection ==="
run_test "City name" \
    "I live in New York City" \
    "LOCATION"

run_test "Full address context" \
    "John Smith lives in San Francisco, California" \
    "PERSON,LOCATION"

# Test 7: IP Address
echo ""
echo "=== IP Address Detection ==="
run_test "IPv4 address" \
    "Server IP is 192.168.1.100" \
    "IP_ADDRESS"

# Test 8: No PII (negative test)
echo ""
echo "=== Negative Tests (No PII) ==="
run_test "Generic text (no PII)" \
    "The weather is nice today" \
    "" \
    "The weather is nice today"

# Summary
echo ""
echo "================================================"
echo "  Test Results"
echo "================================================"
echo ""
echo "  Passed: $PASSED / $TOTAL"
echo "  Failed: $FAILED / $TOTAL"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "  ${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "  ${RED}Some tests failed.${NC}"
    exit 1
fi
