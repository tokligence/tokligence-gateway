#!/usr/bin/env bash
#
# Master Test Runner for Scheduler Integration Tests
#
# Runs all scheduler integration tests in sequence and reports results
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATEWAY_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
cd "${GATEWAY_ROOT}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

echo -e "${BOLD}${CYAN}"
echo "========================================"
echo "Scheduler Integration Test Suite"
echo "========================================"
echo -e "${NC}"
echo "Gateway: Tokligence Gateway v0.3.0"
echo "Feature: Priority-Based Request Scheduler"
echo "Test Directory: ${SCRIPT_DIR}"
echo ""

# Track results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Test execution function
run_test() {
    local test_name=$1
    local test_cmd=$2
    local test_type=$3 # "bash" or "go"

    echo -e "${BOLD}========================================${NC}"
    echo -e "${BOLD}Running: ${test_name}${NC}"
    echo -e "${BOLD}========================================${NC}"
    echo ""

    TOTAL_TESTS=$((TOTAL_TESTS + 1))

    if [ "$test_type" = "bash" ]; then
        if bash "${SCRIPT_DIR}/${test_cmd}"; then
            echo -e "${GREEN}✓ PASSED: ${test_name}${NC}\n"
            PASSED_TESTS=$((PASSED_TESTS + 1))
            return 0
        else
            echo -e "${RED}✗ FAILED: ${test_name}${NC}\n"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            return 1
        fi
    elif [ "$test_type" = "go" ]; then
        # Build and run Go test
        test_binary="/tmp/$(basename ${test_cmd} .go)"
        if go build -o "${test_binary}" "${SCRIPT_DIR}/${test_cmd}"; then
            if "${test_binary}"; then
                echo -e "${GREEN}✓ PASSED: ${test_name}${NC}\n"
                PASSED_TESTS=$((PASSED_TESTS + 1))
                rm -f "${test_binary}"
                return 0
            else
                echo -e "${RED}✗ FAILED: ${test_name}${NC}\n"
                FAILED_TESTS=$((FAILED_TESTS + 1))
                rm -f "${test_binary}"
                return 1
            fi
        else
            echo -e "${RED}✗ BUILD FAILED: ${test_name}${NC}\n"
            FAILED_TESTS=$((FAILED_TESTS + 1))
            return 1
        fi
    fi
}

# Cleanup before tests
echo "Cleaning up previous test artifacts..."
pkill -f "gatewayd" || true
pkill -f "scheduler_demo" || true
rm -f /tmp/gateway_*.log
rm -f /tmp/scheduler_*.log
sleep 1
echo -e "${GREEN}✓ Cleanup complete${NC}\n"

# ========================================
# Test Suite 1: Backward Compatibility
# ========================================
echo -e "${BOLD}${BLUE}Test Suite 1: Backward Compatibility${NC}"
echo -e "${BLUE}Purpose: Verify gateway works when scheduler is disabled${NC}\n"

run_test \
    "Test 1.1: Scheduler Disabled Mode" \
    "test_scheduler_disabled.sh" \
    "bash" || true

# ========================================
# Test Suite 2: Core Scheduler Functionality
# ========================================
echo -e "${BOLD}${BLUE}Test Suite 2: Core Scheduler Functionality${NC}"
echo -e "${BLUE}Purpose: Verify basic scheduler operations${NC}\n"

run_test \
    "Test 2.1: All Priority Levels (P0-P9)" \
    "test_priority_levels.sh" \
    "bash" || true

# ========================================
# Test Suite 3: Capacity Management
# ========================================
echo -e "${BOLD}${BLUE}Test Suite 3: Capacity Management${NC}"
echo -e "${BLUE}Purpose: Verify capacity limits and queueing${NC}\n"

run_test \
    "Test 3.1: Capacity Limits & Queueing" \
    "test_capacity_limits.go" \
    "go" || true

# ========================================
# Test Suite 4: Scheduling Policies
# ========================================
echo -e "${BOLD}${BLUE}Test Suite 4: Scheduling Policies${NC}"
echo -e "${BLUE}Purpose: Verify WFQ fairness and weighted bandwidth${NC}\n"

run_test \
    "Test 4.1: WFQ Fairness (No Starvation)" \
    "test_wfq_fairness.go" \
    "go" || true

# ========================================
# Test Suite 5: Error Handling
# ========================================
echo -e "${BOLD}${BLUE}Test Suite 5: Error Handling & Edge Cases${NC}"
echo -e "${BLUE}Purpose: Verify timeout, rejection, and overload scenarios${NC}\n"

run_test \
    "Test 5.1: Queue Timeout & Rejection" \
    "test_queue_timeout.go" \
    "go" || true

# ========================================
# Test Suite 6: Time-Based Dynamic Rules (Phase 3)
# ========================================
echo -e "${BOLD}${BLUE}Test Suite 6: Time-Based Dynamic Rules${NC}"
echo -e "${BLUE}Purpose: Verify time-based rule engine for dynamic resource allocation${NC}\n"

run_test \
    "Test 6.1: Basic Time Rules Functionality" \
    "test_time_rules_basic.sh" \
    "bash" || true

run_test \
    "Test 6.2: Time Window Evaluation" \
    "test_time_rules_time_windows.sh" \
    "bash" || true

run_test \
    "Test 6.3: Configuration Validation" \
    "test_time_rules_config_validation.sh" \
    "bash" || true

# ========================================
# Final Summary
# ========================================
echo ""
echo -e "${BOLD}${CYAN}========================================"
echo "Test Suite Summary"
echo "========================================${NC}"
echo ""
echo -e "Total Tests:   ${BOLD}${TOTAL_TESTS}${NC}"
echo -e "Passed:        ${GREEN}${BOLD}${PASSED_TESTS}${NC}"
echo -e "Failed:        ${RED}${BOLD}${FAILED_TESTS}${NC}"
echo -e "Skipped:       ${YELLOW}${BOLD}${SKIPPED_TESTS}${NC}"
echo ""

if [ ${FAILED_TESTS} -eq 0 ]; then
    echo -e "${GREEN}${BOLD}✓ ALL TESTS PASSED${NC}"
    echo ""
    echo "Scheduler is ready for production use with:"
    echo "  ✓ Backward compatibility (can be disabled)"
    echo "  ✓ 10-level priority queue"
    echo "  ✓ Capacity management (tokens/sec, RPS, concurrent)"
    echo "  ✓ WFQ fairness (no starvation)"
    echo "  ✓ Proper error handling (timeout, queue full, context limit)"
    echo "  ✓ Time-based dynamic rules (Phase 3)"
    echo ""
    exit 0
else
    echo -e "${RED}${BOLD}✗ SOME TESTS FAILED${NC}"
    echo ""
    echo "Please review failed tests above and fix issues before deployment."
    echo ""
    exit 1
fi
