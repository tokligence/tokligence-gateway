#!/bin/bash
# Test: Configuration Validation
# Purpose: Test invalid config values fallback to defaults
# Tests: invalid work_mode → fallback to "auto"

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

TESTS_PASSED=0
TESTS_FAILED=0

echo "========================================"
echo "Configuration Validation Test"
echo "========================================"
echo ""

# Backup files
BACKUP_CONFIG="/tmp/gateway.ini.validation.backup"
BACKUP_DOTENV="/tmp/.env.validation.backup"

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

check_work_mode() {
    local expected="$1"
    local description="$2"

    sleep 1
    local LOG_FILE="$PROJECT_ROOT/logs/dev-gatewayd-$(date +%Y-%m-%d).log"

    if [ ! -f "$LOG_FILE" ]; then
        echo -e "${RED}❌ FAIL${NC}: Log file not found"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return
    fi

    if grep -q "work mode: $expected" "$LOG_FILE"; then
        echo -e "${GREEN}✅ PASS${NC}: $description (work mode: $expected)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}❌ FAIL${NC}: Expected work mode: $expected"
        echo "  Checking for 'work mode:' in log:"
        grep "work mode:" "$LOG_FILE" | tail -1 | sed 's/^/    /'
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Test 1: Invalid work_mode in .env
echo "Test 1: Invalid work_mode in .env should fallback to 'auto'"
sed -i 's/^TOKLIGENCE_WORK_MODE=.*/TOKLIGENCE_WORK_MODE=invalid_mode/' "$PROJECT_ROOT/.env"
restart_gateway
check_work_mode "auto" "Invalid .env work_mode → auto"
echo ""

# Test 2: Invalid work_mode in config file
echo "Test 2: Invalid work_mode in gateway.ini should fallback to 'auto'"
# Comment out .env setting
sed -i 's/^TOKLIGENCE_WORK_MODE=/#TOKLIGENCE_WORK_MODE=/' "$PROJECT_ROOT/.env"
# Set invalid value in config
sed -i 's/^work_mode = .*/work_mode = invalid_mode/' "$PROJECT_ROOT/config/dev/gateway.ini"
restart_gateway
check_work_mode "auto" "Invalid config work_mode → auto"
echo ""

# Test 3: Empty work_mode
echo "Test 3: Empty work_mode should fallback to 'auto'"
sed -i 's/^work_mode = .*/work_mode = /' "$PROJECT_ROOT/config/dev/gateway.ini"
restart_gateway
check_work_mode "auto" "Empty work_mode → auto"
echo ""

echo "========================================"
echo "Test Results"
echo "========================================"
echo "Passed: $TESTS_PASSED"
echo "Failed: $TESTS_FAILED"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All config validation tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
