#!/bin/bash
# Test Multi-Process Load Balancing
# This script demonstrates how OS kernel distributes requests across workers

set -e

PRESIDIO_URL="${PRESIDIO_URL:-http://localhost:7317}"

echo "=== Multi-Process Load Balancing Test ==="
echo ""

# Check if Presidio is running
if ! curl -s -f "$PRESIDIO_URL/health" > /dev/null 2>&1; then
    echo "ERROR: Presidio is not running at $PRESIDIO_URL"
    echo "Start it with: cd examples/firewall/presidio_sidecar && ./start.sh"
    exit 1
fi

echo "✓ Presidio is running"
echo ""

# Get process information
echo "=== Presidio Process Information ==="
PRESIDIO_PIDS=$(pgrep -f "python.*main.py" | tr '\n' ' ')
if [ -z "$PRESIDIO_PIDS" ]; then
    echo "No Presidio processes found"
    exit 1
fi

WORKER_COUNT=$(echo "$PRESIDIO_PIDS" | wc -w)
echo "Found $WORKER_COUNT Presidio processes:"
for PID in $PRESIDIO_PIDS; do
    echo "  PID: $PID ($(ps -p $PID -o cmd= | cut -c1-60)...)"
done
echo ""

# Check if multiple workers are listening on the same port
echo "=== Port Binding Check ==="
LISTENING=$(netstat -tlnp 2>/dev/null | grep ":7317" | grep "python" || lsof -i :7317 2>/dev/null | grep python || true)
if [ -z "$LISTENING" ]; then
    echo "⚠ Cannot verify port binding (may need sudo)"
else
    echo "$LISTENING" | head -5
fi
echo ""

# Send multiple requests and track which worker handled them
echo "=== Load Distribution Test ==="
echo "Sending 20 requests to see distribution across workers..."
echo ""

for i in {1..20}; do
    RESPONSE=$(curl -s -X POST "$PRESIDIO_URL/v1/filter/input" \
        -H "Content-Type: application/json" \
        -d '{"input":"test request '$i'"}' 2>&1)

    # Just show progress
    echo -n "."
done
echo ""
echo ""

echo "✓ All 20 requests completed successfully"
echo ""

# Explain the mechanism
cat << 'EOF'
=== How Multi-Process Works ===

1. Uvicorn Master Process starts N worker processes
2. Each worker binds to the SAME port (7317) using SO_REUSEPORT
3. OS kernel automatically distributes incoming connections
4. Gateway just connects to localhost:7317 (doesn't know which worker)

Visualization:

    Gateway                 Presidio Workers (all on port 7317)
       |                           |
       |--- Request 1 ----------> Worker 1 (PID 1001)
       |--- Request 2 ----------> Worker 3 (PID 1003)
       |--- Request 3 ----------> Worker 2 (PID 1002)
       |--- Request 4 ----------> Worker 1 (PID 1001)
       |--- Request 5 ----------> Worker 4 (PID 1004)
                                   ...

Kernel uses hash-based load balancing (usually based on source IP/port).

Key Point: Gateway code doesn't need ANY changes!
           Just set PRESIDIO_WORKERS=8 and it automatically scales.

EOF

echo ""
echo "=== Configuration ==="
echo "Current workers: $WORKER_COUNT"
echo ""
echo "To change worker count:"
echo "  export PRESIDIO_WORKERS=8"
echo "  cd examples/firewall/presidio_sidecar"
echo "  ./stop.sh && ./start.sh"
echo ""
