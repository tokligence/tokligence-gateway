#!/bin/bash
# Run all Phase 3 Time-Based Rules integration tests
#
# Usage:
#   ./run_time_rules_tests.sh           # Run all tests
#   ./run_time_rules_tests.sh basic     # Run only basic test
#   ./run_time_rules_tests.sh config    # Run only config validation test
#   ./run_time_rules_tests.sh windows   # Run only time windows test

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

cd "$PROJECT_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "Phase 3: Time-Based Rules Integration Tests"
echo "========================================="
echo

# Build if needed
if [ ! -f "bin/gatewayd" ]; then
    echo "Building gatewayd..."
    make bgd
    echo
fi

# Test selection
RUN_BASIC=true
RUN_CONFIG=true
RUN_WINDOWS=true

if [ "$1" = "basic" ]; then
    RUN_CONFIG=false
    RUN_WINDOWS=false
elif [ "$1" = "config" ]; then
    RUN_BASIC=false
    RUN_WINDOWS=false
elif [ "$1" = "windows" ]; then
    RUN_BASIC=false
    RUN_CONFIG=false
fi

FAILED_TESTS=()
PASSED_TESTS=()

# Helper function to run test
run_test() {
    local test_name=$1
    local test_script=$2

    echo "-------------------------------------------"
    echo "Running: $test_name"
    echo "-------------------------------------------"

    if bash "$test_script"; then
        echo -e "${GREEN}✓ PASSED${NC}: $test_name"
        PASSED_TESTS+=("$test_name")
    else
        echo -e "${RED}✗ FAILED${NC}: $test_name"
        FAILED_TESTS+=("$test_name")
    fi
    echo
}

# Run tests
if [ "$RUN_BASIC" = true ]; then
    run_test "Basic Functionality" "$SCRIPT_DIR/test_time_rules_basic.sh"
fi

if [ "$RUN_CONFIG" = true ]; then
    run_test "Configuration Validation" "$SCRIPT_DIR/test_time_rules_config_validation.sh"
fi

if [ "$RUN_WINDOWS" = true ]; then
    run_test "Time Window Evaluation" "$SCRIPT_DIR/test_time_rules_time_windows.sh"
fi

# Summary
echo "========================================="
echo "Test Summary"
echo "========================================="
echo "Passed: ${#PASSED_TESTS[@]}"
echo "Failed: ${#FAILED_TESTS[@]}"
echo

if [ ${#PASSED_TESTS[@]} -gt 0 ]; then
    echo -e "${GREEN}Passed tests:${NC}"
    for test in "${PASSED_TESTS[@]}"; do
        echo "  ✓ $test"
    done
    echo
fi

if [ ${#FAILED_TESTS[@]} -gt 0 ]; then
    echo -e "${RED}Failed tests:${NC}"
    for test in "${FAILED_TESTS[@]}"; do
        echo "  ✗ $test"
    done
    echo
    exit 1
fi

echo -e "${GREEN}All tests passed!${NC}"
exit 0
