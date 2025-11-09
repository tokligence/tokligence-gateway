#!/bin/bash
# Test: Configuration Hierarchy
# Purpose: Verify configuration priority: env vars > gateway.ini > setting.ini > defaults
# Requirements: Gateway binaries built

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0

# Backup original config
BACKUP_CONFIG="/tmp/gateway_config_backup.ini"
BACKUP_DOTENV="/tmp/gateway_dotenv_backup"

# Cleanup function
cleanup() {
    # Restore original config if exists
    if [ -f "$BACKUP_CONFIG" ]; then
        cp "$BACKUP_CONFIG" "$PROJECT_ROOT/config/dev/gateway.ini"
    fi

    # Restore .env file
    if [ -f "$BACKUP_DOTENV" ]; then
        cp "$BACKUP_DOTENV" "$PROJECT_ROOT/.env"
    fi

    # Restart gateway with original config
    make gfr > /dev/null 2>&1 || true
    sleep 2

    rm -f "$BACKUP_CONFIG" "$BACKUP_DOTENV"
}

trap cleanup EXIT

# Helper to restart gateway and wait
restart_gateway() {
    make gfr > /dev/null 2>&1
    sleep 3  # Give it time to fully start
}

# Helper function to check work mode from logs
check_work_mode() {
    local expected_mode="$1"
    local description="$2"
    # make gfr deletes dev-gatewayd logs, so use the plain gatewayd log
    local log_file="logs/gatewayd-$(date +%Y-%m-%d).log"

    # Check the most recent log entry for work mode
    if tail -100 "$log_file" 2>/dev/null | grep -q "work mode: $expected_mode"; then
        echo -e "${GREEN}✅ PASS${NC}: $description - work_mode=$expected_mode"
        ((TESTS_PASSED++))
        return 0
    else
        echo -e "${RED}❌ FAIL${NC}: $description - expected work_mode=$expected_mode"
        echo "Recent log excerpt:"
        tail -50 "$log_file" 2>/dev/null | grep -i "work" | head -5
        ((TESTS_FAILED++))
        return 1
    fi
}

echo "========================================"
echo "Configuration Hierarchy Test"
echo "========================================"
echo ""

# Build if needed
if [ ! -f "./bin/gatewayd" ]; then
    echo "Building gateway..."
    make build
fi

# Backup current config and .env
cp "$PROJECT_ROOT/config/dev/gateway.ini" "$BACKUP_CONFIG"
if [ -f "$PROJECT_ROOT/.env" ]; then
    cp "$PROJECT_ROOT/.env" "$BACKUP_DOTENV"
fi

echo "Testing configuration hierarchy..."
echo ""

# Test 1: Config file specifies work_mode=auto, .env commented out
echo "Test 1: work_mode=auto from config file (.env disabled)"
cat > "$PROJECT_ROOT/config/dev/gateway.ini" << 'EOF'
[gateway]
enable_auth = false
multiport_mode = false
facade_port = 8081
work_mode = auto
EOF

# Comment out TOKLIGENCE_WORK_MODE in .env
sed -i 's/^TOKLIGENCE_WORK_MODE=/#TOKLIGENCE_WORK_MODE=/' "$PROJECT_ROOT/.env"

restart_gateway
check_work_mode "auto" "work_mode=auto from config file"

sleep 1

# Test 2: Config file specifies work_mode=translation
echo ""
echo "Test 2: work_mode=translation from config file"
cat > "$PROJECT_ROOT/config/dev/gateway.ini" << 'EOF'
[gateway]
enable_auth = false
multiport_mode = false
facade_port = 8081
work_mode = translation
EOF

sed -i 's/^TOKLIGENCE_WORK_MODE=/#TOKLIGENCE_WORK_MODE=/' "$PROJECT_ROOT/.env"
restart_gateway
check_work_mode "translation" "work_mode=translation from config file"

sleep 1

# Test 3: .env TOKLIGENCE_WORK_MODE=passthrough overrides config translation
echo ""
echo "Test 3: .env TOKLIGENCE_WORK_MODE=passthrough overrides config translation"
cat > "$PROJECT_ROOT/config/dev/gateway.ini" << 'EOF'
[gateway]
enable_auth = false
multiport_mode = false
facade_port = 8081
work_mode = translation
EOF

# Set in .env file (highest priority)
sed -i 's/^#*TOKLIGENCE_WORK_MODE=.*/TOKLIGENCE_WORK_MODE=passthrough/' "$PROJECT_ROOT/.env"
restart_gateway
check_work_mode "passthrough" ".env variable overrides config file"

sleep 1

# Test 4: Config file with passthrough mode
echo ""
echo "Test 4: work_mode=passthrough from config"
cat > "$PROJECT_ROOT/config/dev/gateway.ini" << 'EOF'
[gateway]
enable_auth = false
multiport_mode = false
facade_port = 8081
work_mode = passthrough
EOF

sed -i 's/^TOKLIGENCE_WORK_MODE=/#TOKLIGENCE_WORK_MODE=/' "$PROJECT_ROOT/.env"
restart_gateway
check_work_mode "passthrough" "work_mode=passthrough from config"

sleep 1

# Test 5: .env auto beats config passthrough
echo ""
echo "Test 5: .env TOKLIGENCE_WORK_MODE=auto overrides config passthrough"
cat > "$PROJECT_ROOT/config/dev/gateway.ini" << 'EOF'
[gateway]
enable_auth = false
multiport_mode = false
facade_port = 8081
work_mode = passthrough
EOF

sed -i 's/^#*TOKLIGENCE_WORK_MODE=.*/TOKLIGENCE_WORK_MODE=auto/' "$PROJECT_ROOT/.env"
restart_gateway
check_work_mode "auto" ".env variable auto overrides config passthrough"

echo ""
echo "========================================"
echo "Configuration Hierarchy Test Results"
echo "========================================"
echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All configuration hierarchy tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
