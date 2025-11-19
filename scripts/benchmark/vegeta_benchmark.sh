#!/bin/bash
#
# Vegeta-based High-Performance Benchmark
#
# This script uses vegeta (https://github.com/tsenart/vegeta) for extreme
# performance testing. Vegeta is written in Go and can generate much higher
# load than Python-based tools.
#
# Usage:
#   ./scripts/benchmark/vegeta_benchmark.sh [duration] [rate]
#
#   duration - Test duration (default: 30s)
#   rate     - Requests per second (default: 1000)
#
# Example:
#   ./vegeta_benchmark.sh 60s 2000  # 2000 RPS for 60 seconds
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RESULTS_DIR="$PROJECT_ROOT/benchmark-results"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)

DURATION="${1:-30s}"
RATE="${2:-1000}"
TARGET_URL="http://localhost:8081/v1/chat/completions"

echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Tokligence Gateway Vegeta Benchmark               ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${GREEN}Target URL:${NC} $TARGET_URL"
echo -e "${GREEN}Duration:${NC} $DURATION"
echo -e "${GREEN}Rate:${NC} $RATE req/sec"
echo ""

# Check dependencies
echo -e "${YELLOW}[1/3]${NC} Checking dependencies..."

if ! command -v vegeta &> /dev/null; then
    echo -e "${YELLOW}vegeta not found. Installing...${NC}"
    if command -v go &> /dev/null; then
        go install github.com/tsenart/vegeta@latest
        export PATH="$PATH:$(go env GOPATH)/bin"
    else
        echo -e "${RED}Error: Go is required to install vegeta${NC}"
        echo "Install manually: https://github.com/tsenart/vegeta"
        exit 1
    fi
fi

echo -e "${GREEN}✓ vegeta installed${NC}"
echo ""

# Create results directory
mkdir -p "$RESULTS_DIR"

# Prepare target file
TARGET_FILE="$RESULTS_DIR/vegeta-targets-$TIMESTAMP.txt"
cat > "$TARGET_FILE" << 'EOF'
POST http://localhost:8081/v1/chat/completions
Content-Type: application/json
Authorization: Bearer test

{"model":"loopback","messages":[{"role":"user","content":"Hello"}],"max_tokens":100}
EOF

echo -e "${YELLOW}[2/3]${NC} Running vegeta attack..."
echo ""

OUTPUT_FILE="$RESULTS_DIR/vegeta-results-$TIMESTAMP.bin"

# Run vegeta attack
cat "$TARGET_FILE" | vegeta attack \
    -duration="$DURATION" \
    -rate="$RATE" \
    -timeout=10s \
    -workers=100 \
    > "$OUTPUT_FILE"

echo ""
echo -e "${GREEN}✓ Attack complete${NC}"
echo ""

# Generate reports
echo -e "${YELLOW}[3/3]${NC} Generating reports..."

REPORT_TXT="$RESULTS_DIR/vegeta-report-$TIMESTAMP.txt"
PLOT_HTML="$RESULTS_DIR/vegeta-plot-$TIMESTAMP.html"

# Text report
vegeta report "$OUTPUT_FILE" > "$REPORT_TXT"

# HTML plot
vegeta plot "$OUTPUT_FILE" > "$PLOT_HTML"

echo -e "${GREEN}✓ Reports generated${NC}"
echo ""

# Display summary
echo -e "${BLUE}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║                    Results Summary                     ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════╝${NC}"
echo ""

cat "$REPORT_TXT"

echo ""
echo -e "${GREEN}Results saved to:${NC}"
echo "  Report:    $REPORT_TXT"
echo "  Plot:      $PLOT_HTML"
echo "  Raw data:  $OUTPUT_FILE"
echo ""

# Compare with LiteLLM benchmarks
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

echo -e "${GREEN}✓ Benchmark complete!${NC}"
echo ""
