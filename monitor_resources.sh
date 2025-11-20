#!/bin/bash
# Resource monitoring script for gatewayd - for load testing reports

PID_FILE=".tmp/gatewayd.pid"
OUTPUT_FILE="resource_monitor.csv"
INTERVAL=1

if [ ! -f "$PID_FILE" ]; then
    echo "Error: PID file not found at $PID_FILE"
    exit 1
fi

PID=$(cat "$PID_FILE")

if ! kill -0 "$PID" 2>/dev/null; then
    echo "Error: Process with PID $PID is not running"
    exit 1
fi

echo "Monitoring gatewayd (PID: $PID)"
echo "Output: $OUTPUT_FILE"
echo "Press Ctrl+C to stop"
echo ""

# CSV header
echo "timestamp,cpu_percent,mem_rss_mb,mem_percent,threads,open_files,tcp_connections" > "$OUTPUT_FILE"

trap 'echo ""; echo "Stopped. Data saved to $OUTPUT_FILE"; exit 0' INT

while true; do
    TIMESTAMP=$(date +"%Y-%m-%d %H:%M:%S")
    PS_OUTPUT=$(ps -p "$PID" -o %cpu,%mem,rss,nlwp --no-headers 2>/dev/null)

    if [ -z "$PS_OUTPUT" ]; then
        echo "Process stopped"
        break
    fi

    read -r CPU_PERCENT MEM_PERCENT RSS_KB THREADS <<< "$PS_OUTPUT"
    MEM_RSS_MB=$(echo "scale=2; $RSS_KB / 1024" | bc)
    OPEN_FILES=$(ls -1 /proc/$PID/fd 2>/dev/null | wc -l)
    TCP_CONNECTIONS=$(ss -tnp 2>/dev/null | grep -c "pid=$PID" || echo "0")

    echo "$TIMESTAMP,$CPU_PERCENT,$MEM_RSS_MB,$MEM_PERCENT,$THREADS,$OPEN_FILES,$TCP_CONNECTIONS" >> "$OUTPUT_FILE"

    printf "\r[%s] CPU: %5s%% | Mem: %7s MB (%5s%%) | Threads: %3s | FDs: %4s | TCP: %4s" \
        "$TIMESTAMP" "$CPU_PERCENT" "$MEM_RSS_MB" "$MEM_PERCENT" "$THREADS" "$OPEN_FILES" "$TCP_CONNECTIONS"

    sleep "$INTERVAL"
done
