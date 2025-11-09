#!/bin/bash
# Test: Configurable Build Timezone
# Tests that BUILD_TZ environment variable correctly sets build timestamp timezone

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cd "$PROJECT_ROOT"

echo "=== Test: Configurable Build Timezone ==="
echo

# Test 1: Default timezone (Asia/Singapore)
echo "[Test 1] Building with default timezone (Asia/Singapore)..."
make build >/dev/null 2>&1
DEFAULT_VERSION=$(./bin/gatewayd --version 2>&1 | grep "Built at:" || echo "")
echo "Default build output: $DEFAULT_VERSION"

if [[ "$DEFAULT_VERSION" == *"+08"* ]] || [[ "$DEFAULT_VERSION" == *"Asia/Singapore"* ]]; then
    echo "✅ Default timezone shows +08 offset (Asia/Singapore)"
else
    echo "⚠️  Default timezone might not be Asia/Singapore: $DEFAULT_VERSION"
fi
echo

# Test 2: Custom timezone (America/New_York)
echo "[Test 2] Building with BUILD_TZ=America/New_York..."
BUILD_TZ=America/New_York make build >/dev/null 2>&1
NY_VERSION=$(./bin/gatewayd --version 2>&1 | grep "Built at:" || echo "")
echo "New York build output: $NY_VERSION"

if [[ "$NY_VERSION" == *"-05"* ]] || [[ "$NY_VERSION" == *"-04"* ]]; then
    echo "✅ America/New_York timezone shows -05 or -04 offset (EST/EDT)"
else
    echo "❌ FAIL: America/New_York timezone not detected: $NY_VERSION"
    exit 1
fi
echo

# Test 3: UTC timezone
echo "[Test 3] Building with BUILD_TZ=UTC..."
BUILD_TZ=UTC make build >/dev/null 2>&1
UTC_VERSION=$(./bin/gatewayd --version 2>&1 | grep "Built at:" || echo "")
echo "UTC build output: $UTC_VERSION"

if [[ "$UTC_VERSION" == *"+00"* ]] || [[ "$UTC_VERSION" == *"Z"* ]]; then
    echo "✅ UTC timezone shows +00 offset or Z suffix"
else
    echo "❌ FAIL: UTC timezone not detected: $UTC_VERSION"
    exit 1
fi
echo

# Test 4: Verify timestamps are different (due to build time)
echo "[Test 4] Verifying all builds have timestamps..."
./bin/gatewayd --version
echo

if [[ -n "$DEFAULT_VERSION" ]] && [[ -n "$NY_VERSION" ]] && [[ -n "$UTC_VERSION" ]]; then
    echo "✅ All builds include timestamp information"
else
    echo "❌ FAIL: Some builds missing timestamp"
    exit 1
fi

echo
echo "=== All Build Timezone Tests Passed ==="
