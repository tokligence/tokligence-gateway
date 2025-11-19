#!/bin/bash
#
# Tokligence Gateway Performance Benchmark Runner
#
# This script runs a comprehensive performance benchmark matching LiteLLM's methodology:
# - 4 CPU cores, 8GB RAM (Docker constrained)
# - 1000 concurrent users
# - 500 user/sec spawn rate
# - 5 minute test duration
# - Loopback adapter (no external API latency)
#
# Usage:
#   ./scripts/benchmark/run_benchmark.sh [quick|full|stress]
#
#   quick  - 1 min test, 100 users (default)
#   full   - 5 min test, 1000 users (LiteLLM comparison)
#   stress - 10 min test, 2000 users (stress test)

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$PROJECT_ROOT/benchmark-results"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

# Test profiles
TEST_PROFILE="${1:-quick}"

case "$TEST_PROFILE" in
    quick)
        USERS=500
        SPAWN_RATE=250
        RUN_TIME="1m"
        WORKERS=4
        ;;
    full)
        USERS=2000
        SPAWN_RATE=1000
        RUN_TIME="5m"
        WORKERS=8
        ;;
    stress)
        USERS=4000
        SPAWN_RATE=2000
        RUN_TIME="10m"
        WORKERS=16
        ;;
    *)
        echo -e "${RED}Invalid test profile: $TEST_PROFILE${NC}"
        echo "Usage: $0 [quick|full|stress]"
        exit 1
        ;;
esac

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Tokligence Gateway Performance Benchmark          ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}Profile:${NC} $TEST_PROFILE"
echo -e "${GREEN}Users:${NC} $USERS"
echo -e "${GREEN}Spawn Rate:${NC} $SPAWN_RATE/sec"
echo -e "${GREEN}Duration:${NC} $RUN_TIME"
echo -e "${GREEN}Workers:${NC} $WORKERS (parallel Locust processes)"
echo ""

# Create results directory
mkdir -p "$RESULTS_DIR"

# Step 1: Check dependencies
echo -e "${YELLOW}[1/6]${NC} Checking dependencies..."

if ! command -v docker &> /dev/null; then
    echo -e "${RED}Error: docker not found${NC}"
    exit 1
fi

if ! command -v python3 &> /dev/null; then
    echo -e "${RED}Error: python3 not found${NC}"
    exit 1
fi

# Setup Python virtual environment
VENV_DIR="$SCRIPT_DIR/venv"
if [ ! -d "$VENV_DIR" ]; then
    echo -e "${YELLOW}Creating Python virtual environment...${NC}"
    python3 -m venv "$VENV_DIR"
fi

# Activate venv and install dependencies
source "$VENV_DIR/bin/activate"

if ! python -c "import locust" &> /dev/null; then
    echo -e "${YELLOW}Installing Python dependencies...${NC}"
    pip install -q -r "$SCRIPT_DIR/requirements.txt"
fi

echo -e "${GREEN}✓ Dependencies OK${NC}"
echo ""

# Step 2: Build Docker image
echo -e "${YELLOW}[2/6]${NC} Building Docker image..."

cd "$PROJECT_ROOT"

# Check if we should rebuild
if docker images | grep -q "tokligence-gateway-bench"; then
    echo -e "${GREEN}✓ Using existing image${NC}"
else
    docker build -t tokligence-gateway-bench -f docker/Dockerfile.personal .
fi

echo ""

# Step 3: Stop existing containers
echo -e "${YELLOW}[3/6]${NC} Cleaning up existing containers..."

if docker ps -a | grep -q gateway-bench; then
    docker rm -f gateway-bench 2>/dev/null || true
fi

echo -e "${GREEN}✓ Cleanup complete${NC}"
echo ""

# Step 4: Start gateway in Docker
echo -e "${YELLOW}[4/6]${NC} Starting gateway (4 CPU, 8GB RAM)..."

# Detect available CPUs and dedicate 4 cores
TOTAL_CPUS=$(nproc)
if [ "$TOTAL_CPUS" -ge 8 ]; then
    # Use CPUs 4-7 for dedicated isolation (assuming 0-3 for system)
    CPUSET="4-7"
    echo -e "${GREEN}Using dedicated CPUs: $CPUSET (isolated from CPU 0-3)${NC}"
else
    # Fall back to cpus quota if less than 8 cores
    CPUSET=""
    echo -e "${YELLOW}Warning: Less than 8 cores available. Using quota-based limiting.${NC}"
fi

if [ -n "$CPUSET" ]; then
    docker run -d \
        --name gateway-bench \
        --cpuset-cpus="$CPUSET" \
        --memory=8g \
        --memory-swap=8g \
        --memory-swappiness=0 \
        -p 8081:8081 \
        -e TOKLIGENCE_AUTH_DISABLED=true \
        -e TOKLIGENCE_MARKETPLACE_ENABLED=false \
        -e TOKLIGENCE_LOG_LEVEL=info \
        -e TOKLIGENCE_TELEMETRY_ENABLED=false \
        -e TOKLIGENCE_CHAT_TO_ANTHROPIC=false \
        -e TOKLIGENCE_ROUTES="loopback=>loopback" \
        tokligence-gateway-bench
else
    docker run -d \
        --name gateway-bench \
        --cpus=4 \
        --memory=8g \
        --memory-swap=8g \
        --memory-swappiness=0 \
        -p 8081:8081 \
        -e TOKLIGENCE_AUTH_DISABLED=true \
        -e TOKLIGENCE_MARKETPLACE_ENABLED=false \
        -e TOKLIGENCE_LOG_LEVEL=info \
        -e TOKLIGENCE_TELEMETRY_ENABLED=false \
        -e TOKLIGENCE_CHAT_TO_ANTHROPIC=false \
        -e TOKLIGENCE_ROUTES="loopback=>loopback" \
        tokligence-gateway-bench
fi

# Wait for gateway to start
echo -n "Waiting for gateway to be ready"
for i in {1..30}; do
    if curl -s http://localhost:8081/health > /dev/null 2>&1; then
        echo ""
        echo -e "${GREEN}✓ Gateway ready${NC}"
        break
    fi
    echo -n "."
    sleep 1
done

if ! curl -s http://localhost:8081/health > /dev/null 2>&1; then
    echo -e "${RED}Error: Gateway failed to start${NC}"
    docker logs gateway-bench
    exit 1
fi

echo ""

# Step 5: Run load test
echo -e "${YELLOW}[5/6]${NC} Running load test ($RUN_TIME with $USERS users)..."
echo ""

OUTPUT_PREFIX="$RESULTS_DIR/benchmark-$TEST_PROFILE-$TIMESTAMP"

locust \
    -f "$SCRIPT_DIR/locustfile.py" \
    --host=http://localhost:8081 \
    --users=$USERS \
    --spawn-rate=$SPAWN_RATE \
    --run-time=$RUN_TIME \
    --processes=$WORKERS \
    --headless \
    --html="$OUTPUT_PREFIX-report.html" \
    --csv="$OUTPUT_PREFIX" \
    --loglevel=INFO

echo ""
echo -e "${GREEN}✓ Load test complete${NC}"
echo ""

# Step 6: Collect metrics
echo -e "${YELLOW}[6/6]${NC} Collecting results..."

# Get Docker stats
docker stats --no-stream gateway-bench > "$OUTPUT_PREFIX-docker-stats.txt"

# Get gateway logs
docker logs gateway-bench > "$OUTPUT_PREFIX-gateway.log" 2>&1

# Stop container
docker stop gateway-bench > /dev/null 2>&1
docker rm gateway-bench > /dev/null 2>&1

echo -e "${GREEN}✓ Results collected${NC}"
echo ""

# Display summary
echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    Results Summary                     ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

# Parse CSV stats
if [ -f "$OUTPUT_PREFIX\_stats.csv" ]; then
    echo -e "${GREEN}Detailed Results:${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # Extract key metrics from CSV
    python3 << EOF
import csv
import sys

try:
    with open('$OUTPUT_PREFIX\_stats.csv', 'r') as f:
        reader = csv.DictReader(f)
        total_stats = None
        for row in reader:
            if row['Name'] == 'Aggregated':
                total_stats = row
                break

        if total_stats:
            print(f"Total Requests:      {total_stats['Request Count']}")
            print(f"Total Failures:      {total_stats['Failure Count']}")
            print(f"Requests/sec:        {total_stats['Requests/s']}")
            print(f"Median Latency:      {total_stats['Median Response Time']} ms")
            print(f"Average Latency:     {total_stats['Average Response Time']} ms")
            print(f"Min Latency:         {total_stats['Min Response Time']} ms")
            print(f"Max Latency:         {total_stats['Max Response Time']} ms")

            # Calculate P95 and P99 if available
            if '95%' in total_stats:
                print(f"P95 Latency:         {total_stats['95%']} ms")
            if '99%' in total_stats:
                print(f"P99 Latency:         {total_stats['99%']} ms")

            # Error rate
            requests = int(total_stats['Request Count'])
            failures = int(total_stats['Failure Count'])
            if requests > 0:
                error_rate = (failures / requests) * 100
                print(f"Error Rate:          {error_rate:.2f}%")
        else:
            print("No aggregated statistics found")
except Exception as e:
    print(f"Error parsing results: {e}", file=sys.stderr)
EOF

    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
fi

echo ""
echo -e "${GREEN}Results saved to:${NC}"
echo "  Report:       $OUTPUT_PREFIX-report.html"
echo "  CSV Stats:    $OUTPUT_PREFIX\_stats.csv"
echo "  Docker Stats: $OUTPUT_PREFIX-docker-stats.txt"
echo "  Gateway Logs: $OUTPUT_PREFIX-gateway.log"
echo ""

# Compare with LiteLLM if full benchmark
if [ "$TEST_PROFILE" = "full" ]; then
    echo -e "${BLUE}Comparison with LiteLLM Benchmarks:${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "Metric              LiteLLM (4inst)   Target"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "RPS                 1,170            >1,000"
    echo "Median Latency      100 ms           <120 ms"
    echo "P95 Latency         150 ms           <180 ms"
    echo "P99 Latency         240 ms           <300 ms"
    echo "Gateway Overhead    2-8 ms           <10 ms"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo -e "${YELLOW}Note:${NC} LiteLLM used 4 instances, Tokligence uses 1 instance"
    echo ""
fi

echo -e "${GREEN}✓ Benchmark complete!${NC}"
echo ""
echo -e "Open the HTML report: ${BLUE}$OUTPUT_PREFIX-report.html${NC}"
echo ""
