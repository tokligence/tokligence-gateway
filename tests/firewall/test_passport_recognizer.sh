#!/bin/bash
# Test passport recognition for 22 countries
# This script tests the PassportRecognizer in the Presidio sidecar
#
# Usage: ./test_passport_recognizer.sh [presidio_url]
# Default: http://localhost:7317

set -e

PRESIDIO_URL="${1:-http://localhost:7317}"
PASSED=0
FAILED=0
TOTAL=0

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=============================================="
echo "  Passport Recognizer Test Suite"
echo "  Testing 22 countries + false positives"
echo "=============================================="
echo ""

# Check if Presidio is running
if ! curl -s "${PRESIDIO_URL}/health" > /dev/null 2>&1; then
    echo -e "${RED}ERROR: Presidio sidecar not running at ${PRESIDIO_URL}${NC}"
    echo "Start it with: cd examples/firewall/presidio_sidecar && python main.py"
    exit 1
fi

echo -e "Presidio sidecar: ${GREEN}Running${NC}"
echo ""

# Test function for passport detection
test_passport() {
    local country="$1"
    local passport="$2"
    local expected="${3:-PASS}"  # PASS or FAIL

    TOTAL=$((TOTAL + 1))

    local result=$(curl -s -X POST "${PRESIDIO_URL}/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d "{\"input\": \"$passport\"}")

    local detected=$(echo "$result" | python3 -c "
import sys, json
d = json.load(sys.stdin)
detected = any(e.get('type') == 'PASSPORT' for e in d.get('entities', []))
print('PASS' if detected else 'FAIL')
" 2>/dev/null || echo "ERROR")

    if [[ "$detected" == "$expected" ]]; then
        PASSED=$((PASSED + 1))
        printf "${GREEN}%-6s${NC} %-20s %s\n" "OK" "$country" "$passport"
    else
        FAILED=$((FAILED + 1))
        printf "${RED}%-6s${NC} %-20s %s (expected: %s, got: %s)\n" "FAIL" "$country" "$passport" "$expected" "$detected"
    fi
}

# Test function for false positive check (should NOT detect passport)
test_no_passport() {
    local desc="$1"
    local text="$2"

    TOTAL=$((TOTAL + 1))

    local result=$(curl -s -X POST "${PRESIDIO_URL}/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d "{\"input\": \"$text\"}")

    local detected=$(echo "$result" | python3 -c "
import sys, json
d = json.load(sys.stdin)
detected = any(e.get('type') == 'PASSPORT' for e in d.get('entities', []))
print('FALSE_POS' if detected else 'OK')
" 2>/dev/null || echo "ERROR")

    if [[ "$detected" == "OK" ]]; then
        PASSED=$((PASSED + 1))
        printf "${GREEN}%-10s${NC} %-25s %s\n" "OK" "$desc" "$text"
    else
        FAILED=$((FAILED + 1))
        printf "${RED}%-10s${NC} %-25s %s\n" "FALSE_POS" "$desc" "$text"
    fi
}

echo "=============================================="
echo "  Part 1: Passport Detection (22 countries)"
echo "=============================================="
echo ""
echo "Status Country              Passport Number"
echo "------ -------------------- --------------------"

# Asia (8 countries)
test_passport "China" "E12345678"
test_passport "China (G)" "G87654321"
test_passport "Japan" "TZ1234567"
test_passport "South Korea" "M12345678"
test_passport "India" "J1234567"
test_passport "Singapore" "S1234567A"
test_passport "Malaysia" "A12345678"
test_passport "Thailand" "AA1234567"
test_passport "Philippines" "EC1234567"

# Europe (6 countries + UK with context)
test_passport "France" "15AB12345"
test_passport "Germany" "CFGHJK123"
test_passport "Italy" "AA1234567"
test_passport "Spain" "AAA123456"
test_passport "Poland" "AY1234567"
test_passport "Russia" "70 1234567"
test_passport "UK (context)" "passport#123456789"

# Americas (4 countries)
test_passport "Canada" "AB123456"
test_passport "Mexico" "G12345678"
test_passport "Brazil" "FH123456"
test_passport "US (context)" "passport: 123456789"

# Oceania (2 countries)
test_passport "Australia" "PA1234567"
test_passport "New Zealand" "LN123456"

# Middle East (2 countries)
test_passport "Saudi Arabia" "A12345678"
test_passport "UAE (context)" "passport 123456789"

echo ""
echo "=============================================="
echo "  Part 2: False Positive Prevention"
echo "=============================================="
echo ""
echo "Status     Description               Test Text"
echo "---------- ------------------------- ----------------------------------"

# Numbers that should NOT match passport
test_no_passport "SSN" "My SSN is 123-45-6789"
test_no_passport "Phone" "+1 555 123 4567"
test_no_passport "IP Address" "Server IP: 192.168.1.100"
test_no_passport "ZIP Code" "ZIP: 12345"
test_no_passport "Random 9 digits" "Order ID: 987654321"
test_no_passport "Random 8 digits" "Reference: 12345678"
test_no_passport "Date (YYYYMMDD)" "Date: 20241225"
test_no_passport "Credit Card last 4" "Card ending in 1234"
test_no_passport "Short code" "Use code ABC123"
test_no_passport "Product SKU" "SKU: ABC-12345"

echo ""
echo "=============================================="
echo "  Test Results Summary"
echo "=============================================="
echo ""
echo "Total tests: $TOTAL"
echo -e "Passed:      ${GREEN}$PASSED${NC}"
if [[ $FAILED -gt 0 ]]; then
    echo -e "Failed:      ${RED}$FAILED${NC}"
    echo ""
    exit 1
else
    echo "Failed:      0"
    echo ""
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
fi
