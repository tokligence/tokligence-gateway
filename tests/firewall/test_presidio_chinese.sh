#!/bin/bash
# Test script for Chinese PII Detection via Presidio
# Tests Chinese person names, phone numbers, and ID cards

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
    echo "  中文 PII 检测测试 (Chinese PII Detection)"
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
    local check_redaction="${4:-}"

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
    local pii_count=$(echo "$response" | jq -r '.annotations.pii_count // 0')
    local redacted=$(echo "$response" | jq -r '.redacted_input // ""')

    local pass=true
    local details=""

    # Check expected types
    if [ -n "$expected_types" ]; then
        for expected_type in $(echo "$expected_types" | tr ',' ' '); do
            if ! echo "$types_found" | grep -q "$expected_type"; then
                pass=false
                details="Expected type '$expected_type' not found. Got: $types_found"
                break
            fi
        done
    fi

    # Check redaction contains mask if specified
    if [ "$pass" = true ] && [ -n "$check_redaction" ]; then
        if ! echo "$redacted" | grep -q "$check_redaction"; then
            pass=false
            details="Expected redaction to contain '$check_redaction'. Got: $redacted"
        fi
    fi

    if [ "$pass" = true ]; then
        echo -e "${GREEN}PASSED${NC} (detected: $types_found)"
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

echo "Running Chinese PII detection tests..."
echo ""

# Test 1: Chinese person names (中文人名)
echo "=== 中文人名检测 (Chinese Name Detection) ==="
run_test "单姓两字名 (2-char name)" \
    "我叫张三，请联系我" \
    "PERSON" \
    "[PERSON]"

run_test "单姓三字名 (3-char name)" \
    "客户是李明华先生" \
    "PERSON" \
    "[PERSON]"

run_test "复姓 (Double surname)" \
    "请找欧阳明月经理" \
    "PERSON" \
    "[PERSON]"

run_test "多个中文名 (Multiple names)" \
    "张三和李四是同事" \
    "PERSON"

# Test 2: Chinese phone numbers (中国手机号)
echo ""
echo "=== 中国手机号检测 (Chinese Phone Detection) ==="
run_test "手机号 (Mobile)" \
    "我的手机是13800138000" \
    "PHONE_NUMBER" \
    "[PHONE]"

run_test "手机号带空格 (Mobile with spaces)" \
    "联系电话：138 0013 8000" \
    "PHONE_NUMBER"

# Test 3: Chinese ID card (身份证)
echo ""
echo "=== 身份证号检测 (ID Card Detection) ==="
run_test "18位身份证 (18-digit ID)" \
    "身份证号：110101199001011234" \
    "CN_ID_CARD" \
    "[身份证]"

run_test "身份证X结尾 (ID ending with X)" \
    "我的身份证是32010119900101123X" \
    "CN_ID_CARD"

# Test 4: Mixed Chinese PII
echo ""
echo "=== 混合检测 (Mixed PII) ==="
run_test "姓名+手机 (Name + Phone)" \
    "张三的手机号是13800138000" \
    "PERSON,PHONE_NUMBER"

run_test "姓名+邮箱 (Name + Email)" \
    "请发邮件给李四 lisi@example.com" \
    "PERSON,EMAIL_ADDRESS"

run_test "全部信息 (All info)" \
    "客户张三，手机13800138000，身份证110101199001011234" \
    "PERSON,PHONE_NUMBER,CN_ID_CARD"

# Test 5: Edge cases
echo ""
echo "=== 边界情况 (Edge Cases) ==="
run_test "非人名词语 (Non-name words)" \
    "黄金价格上涨了" \
    ""

run_test "中英混合 (Mixed CN/EN)" \
    "John和张三是同事，email是john@test.com" \
    "PERSON,EMAIL_ADDRESS"

# Summary
echo ""
echo "================================================"
echo "  测试结果 (Test Results)"
echo "================================================"
echo ""
echo "  通过 (Passed): $PASSED / $TOTAL"
echo "  失败 (Failed): $FAILED / $TOTAL"
echo ""

if [ $FAILED -eq 0 ]; then
    echo -e "  ${GREEN}所有测试通过! (All tests passed!)${NC}"
    exit 0
else
    echo -e "  ${RED}部分测试失败 (Some tests failed)${NC}"
    exit 1
fi
