#!/bin/bash
# Start Presidio Sidecar Service

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="$SCRIPT_DIR/venv"
PID_FILE="$SCRIPT_DIR/presidio.pid"

if [ ! -d "$VENV_DIR" ]; then
    echo "ERROR: Virtual environment not found at: $VENV_DIR"
    echo "Please run ./setup.sh first"
    exit 1
fi

# Check if already running
if [ -f "$PID_FILE" ]; then
    OLD_PID=$(cat "$PID_FILE")
    if ps -p "$OLD_PID" > /dev/null 2>&1; then
        echo "ERROR: Presidio sidecar is already running (PID: $OLD_PID)"
        echo "Stop it first with: ./stop.sh"
        exit 1
    else
        echo "Removing stale PID file..."
        rm -f "$PID_FILE"
    fi
fi

echo "Starting Presidio sidecar service..."

# Activate venv and start service
source "$VENV_DIR/bin/activate"

# Configuration
export PRESIDIO_HOST="${PRESIDIO_HOST:-0.0.0.0}"
export PRESIDIO_PORT="${PRESIDIO_PORT:-7317}"  # Default: 7317 (avoid port conflicts)
export PRESIDIO_WORKERS="${PRESIDIO_WORKERS:-4}"  # Default: 4 workers for multi-core
export PRESIDIO_LOG_LEVEL="${PRESIDIO_LOG_LEVEL:-info}"

echo "Configuration:"
echo "  Host: $PRESIDIO_HOST"
echo "  Port: $PRESIDIO_PORT"
echo "  Workers: $PRESIDIO_WORKERS"

# Start in background
nohup python "$SCRIPT_DIR/main.py" > "$SCRIPT_DIR/presidio.log" 2>&1 &
PID=$!

# Save PID
echo $PID > "$PID_FILE"

# Wait a moment and check if it's running
sleep 2

if ps -p $PID > /dev/null 2>&1; then
    echo "✓ Presidio sidecar started successfully (PID: $PID)"
    echo "  Logs: $SCRIPT_DIR/presidio.log"
    echo "  Health check: curl http://localhost:7317/health"
else
    echo "✗ Failed to start Presidio sidecar"
    echo "Check logs: tail -f $SCRIPT_DIR/presidio.log"
    rm -f "$PID_FILE"
    exit 1
fi
