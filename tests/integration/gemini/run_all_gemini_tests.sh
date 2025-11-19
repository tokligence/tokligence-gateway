#!/usr/bin/env bash
# Run all Gemini integration tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_URL="${GATEWAY_URL:-http://localhost:8084}"
TEST_MODEL="${TEST_MODEL:-gemini-2.0-flash-exp}"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Gemini Integration Test Suite${NC}"
echo -e "${BLUE}========================================${NC}"
echo
echo "Gateway URL: $GATEWAY_URL"
echo "Test Model: $TEST_MODEL"
echo

# Check if gateway is running
echo "Checking gateway connectivity..."
if curl -s -f "${GATEWAY_URL}/health" > /dev/null 2>&1; then
    echo -e "${GREEN}✓${NC} Gateway is reachable"
else
    echo -e "${RED}✗${NC} Gateway is not reachable at $GATEWAY_URL"
    echo "Please start the gateway with: make gfr"
    exit 1
fi
echo

# Check if GEMINI_API_KEY is configured
echo "Checking configuration..."
if [ -z "${TOKLIGENCE_GEMINI_API_KEY:-}" ]; then
    echo -e "${YELLOW}⚠${NC} TOKLIGENCE_GEMINI_API_KEY is not set"
    echo "Some tests may fail due to missing API key"
else
    echo -e "${GREEN}✓${NC} TOKLIGENCE_GEMINI_API_KEY is configured"
fi
echo

# Run test suites
test_suites=(
    "test_native_api.sh"
    "test_openai_compat.sh"
    "test_error_handling.sh"
)

total_passed=0
total_failed=0

for suite in "${test_suites[@]}"; do
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Running: $suite${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo

    if [ -f "$SCRIPT_DIR/$suite" ]; then
        if GATEWAY_URL="$GATEWAY_URL" TEST_MODEL="$TEST_MODEL" bash "$SCRIPT_DIR/$suite"; then
            echo -e "${GREEN}✓ $suite passed${NC}"
            echo
        else
            echo -e "${RED}✗ $suite failed${NC}"
            echo
            ((total_failed++))
        fi
    else
        echo -e "${YELLOW}⚠ $suite not found, skipping${NC}"
        echo
    fi
done

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}  Test Suite Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo

if [ $total_failed -eq 0 ]; then
    echo -e "${GREEN}✓ All test suites passed!${NC}"
    exit 0
else
    echo -e "${RED}✗ $total_failed test suite(s) failed${NC}"
    exit 1
fi
