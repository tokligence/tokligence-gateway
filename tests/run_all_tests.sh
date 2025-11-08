#!/usr/bin/env bash
# Run all gateway tests

set -euo pipefail

echo "=========================================="
echo "Tokligence Gateway Test Suite"
echo "=========================================="
echo ""

# Track results
passed=0
failed=0
skipped=0

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

run_test() {
    local test_file=$1
    local test_name=$(basename "$test_file" .sh)

    echo "Running: $test_name"
    echo "----------------------------------------"

    if bash "$test_file"; then
        echo -e "${GREEN}✅ PASS${NC}: $test_name"
        ((passed++))
    else
        echo -e "${RED}❌ FAIL${NC}: $test_name"
        ((failed++))
    fi
    echo ""
    sleep 1  # Give gateway time between tests
}

# Tool Calls Tests
echo "=== Tool Calls Tests ==="
for test in integration/tool_calls/test_*.sh; do
    [ -f "$test" ] && run_test "$test"
done

# Responses API Tests
echo "=== Responses API Tests ==="
for test in integration/responses_api/test_*.sh; do
    [ -f "$test" ] && run_test "$test"
done

# Streaming Tests
echo "=== Streaming Tests ==="
for test in integration/streaming/test_*.sh; do
    [ -f "$test" ] && run_test "$test"
done

# Duplicate Detection Tests
echo "=== Duplicate Detection Tests ==="
for test in integration/duplicate_detection/test_*.sh; do
    [ -f "$test" ] && run_test "$test"
done

# Configuration Tests
echo "=== Configuration Tests ==="
for test in config/test_*.sh; do
    [ -f "$test" ] && run_test "$test"
done

# Summary
echo "=========================================="
echo "Test Results Summary"
echo "=========================================="
echo -e "${GREEN}Passed:${NC}  $passed"
echo -e "${RED}Failed:${NC}  $failed"
echo -e "${YELLOW}Skipped:${NC} $skipped"
echo "Total:   $((passed + failed + skipped))"
echo ""

if [ $failed -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed${NC}"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    exit 1
fi
