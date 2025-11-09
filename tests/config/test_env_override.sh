#!/bin/bash
# Test: Environment Variable Override
# Purpose: Test .env file variables override config files
# Tests: TOKLIGENCE_WORK_MODE, TOKLIGENCE_ROUTES

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Environment Variable Override Test"
echo "========================================"
echo ""

# Backup files
BACKUP_CONFIG="/tmp/gateway.ini.envtest.backup"
BACKUP_DOTENV="/tmp/.env.envtest.backup"

cp "$PROJECT_ROOT/config/dev/gateway.ini" "$BACKUP_CONFIG"
if [ -f "$PROJECT_ROOT/.env" ]; then
    cp "$PROJECT_ROOT/.env" "$BACKUP_DOTENV"
fi

cleanup() {
    echo ""
    echo "Cleaning up..."
    cp "$BACKUP_CONFIG" "$PROJECT_ROOT/config/dev/gateway.ini"
    if [ -f "$BACKUP_DOTENV" ]; then
        cp "$BACKUP_DOTENV" "$PROJECT_ROOT/.env"
    fi
    cd "$PROJECT_ROOT" && make gfr > /dev/null 2>&1
}

trap cleanup EXIT

restart_gateway() {
    echo "  → Restarting gateway..."
    cd "$PROJECT_ROOT"
    make gfr > /dev/null 2>&1
    sleep 2
}

# Test 1: TOKLIGENCE_WORK_MODE overrides gateway.ini
echo "Test 1: .env TOKLIGENCE_WORK_MODE overrides gateway.ini"

# Set config file to passthrough
sed -i 's/^work_mode = .*/work_mode = passthrough/' "$PROJECT_ROOT/config/dev/gateway.ini"

# Set .env to translation
sed -i 's/^TOKLIGENCE_WORK_MODE=.*/TOKLIGENCE_WORK_MODE=translation/' "$PROJECT_ROOT/.env"

restart_gateway

LOG_FILE="$PROJECT_ROOT/logs/dev-gatewayd-$(date +%Y-%m-%d).log"
if grep -q "work mode: translation" "$LOG_FILE"; then
    echo -e "${GREEN}✅ PASS${NC}: .env overrides config file (work mode: translation)"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: .env did not override config"
    grep "work mode:" "$LOG_FILE" | tail -1 | sed 's/^/  /'
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 2: TOKLIGENCE_ROUTES in .env
echo "Test 2: .env TOKLIGENCE_ROUTES is loaded"

# Check if routes are loaded from .env
if grep -q "routes configured:" "$LOG_FILE"; then
    ROUTES=$(grep "routes configured:" "$LOG_FILE" | tail -1)
    if echo "$ROUTES" | grep -q "claude\*" && echo "$ROUTES" | grep -q "gpt\*"; then
        echo -e "${GREEN}✅ PASS${NC}: Routes loaded from .env"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}❌ FAIL${NC}: Routes not complete"
        echo "  $ROUTES"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
else
    echo -e "${RED}❌ FAIL${NC}: No routes logged"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

# Test 3: Restore to auto and verify
echo "Test 3: Restore to auto mode"
sed -i 's/^TOKLIGENCE_WORK_MODE=.*/TOKLIGENCE_WORK_MODE=auto/' "$PROJECT_ROOT/.env"
restart_gateway

if grep -q "work mode: auto" "$LOG_FILE"; then
    echo -e "${GREEN}✅ PASS${NC}: Successfully restored to auto mode"
    TESTS_PASSED=$((TESTS_PASSED + 1))
else
    echo -e "${RED}❌ FAIL${NC}: Failed to restore"
    TESTS_FAILED=$((TESTS_FAILED + 1))
fi
echo ""

echo "========================================"
echo "Test Results"
echo "========================================"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All env override tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
