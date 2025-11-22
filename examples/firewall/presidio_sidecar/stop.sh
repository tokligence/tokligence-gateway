#!/bin/bash
# Stop Presidio Sidecar Service

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PID_FILE="$SCRIPT_DIR/presidio.pid"

if [ ! -f "$PID_FILE" ]; then
    echo "Presidio sidecar is not running (no PID file found)"
    exit 0
fi

PID=$(cat "$PID_FILE")

if ! ps -p "$PID" > /dev/null 2>&1; then
    echo "Presidio sidecar is not running (stale PID: $PID)"
    rm -f "$PID_FILE"
    exit 0
fi

echo "Stopping Presidio sidecar (PID: $PID)..."
kill "$PID"

# Wait for it to stop
for i in {1..10}; do
    if ! ps -p "$PID" > /dev/null 2>&1; then
        echo "✓ Presidio sidecar stopped"
        rm -f "$PID_FILE"
        exit 0
    fi
    sleep 1
done

# Force kill if still running
if ps -p "$PID" > /dev/null 2>&1; then
    echo "Force killing..."
    kill -9 "$PID"
    sleep 1
fi

rm -f "$PID_FILE"
echo "✓ Presidio sidecar stopped"
