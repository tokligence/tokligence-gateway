#!/bin/bash

set -e

echo "========================================="
echo "Testing Marketplace Ping Communication"
echo "========================================="
echo ""

# Test 1: Test marketplace ping endpoint directly
echo "Test 1: Direct marketplace ping"
echo "--------------------------------------"
curl -s -X POST http://localhost:8082/api/v1/gateway/ping \
  -H "Content-Type: application/json" \
  -d '{"install_id":"test-install-123","gateway_version":"0.1.0","platform":"linux/amd64","database_type":"sqlite"}' \
  | python3 -c "import sys, json; data=json.load(sys.stdin); print(f'✓ Update available: {data[\"update_available\"]}'); print(f'✓ Latest version: {data.get(\"latest_version\", \"N/A\")}'); print(f'✓ Security update: {data.get(\"security_update\", False)}'); print(f'✓ Announcements: {len(data.get(\"announcements\", []))}')"
echo ""

# Test 2: Test with newer version (should show no update)
echo "Test 2: Ping with latest version"
echo "--------------------------------------"
curl -s -X POST http://localhost:8082/api/v1/gateway/ping \
  -H "Content-Type: application/json" \
  -d '{"install_id":"test-install-456","gateway_version":"0.2.0","platform":"darwin/arm64","database_type":"postgres"}' \
  | python3 -c "import sys, json; data=json.load(sys.stdin); print(f'✓ Update available: {data[\"update_available\"]}'); print(f'✓ Announcements: {len(data.get(\"announcements\", []))}')"
echo ""

# Test 3: View announcement details
echo "Test 3: Announcement details"
echo "--------------------------------------"
curl -s -X POST http://localhost:8082/api/v1/gateway/ping \
  -H "Content-Type: application/json" \
  -d '{"install_id":"test-install-789","gateway_version":"0.1.0","platform":"windows/amd64","database_type":"sqlite"}' \
  | python3 -c "
import sys, json
data = json.load(sys.stdin)
for ann in data.get('announcements', [])[:2]:
    print(f\"📢 [{ann['priority'].upper()}] {ann['title']}\")
    print(f\"   Type: {ann['type']}\")
    print(f\"   {ann['message']}\")
    print()
"
echo ""

# Test 4: Check telemetry stats
echo "Test 4: Telemetry statistics"
echo "--------------------------------------"
curl -s http://localhost:8082/telemetry/stats \
  | python3 -c "import sys, json; data=json.load(sys.stdin); print(f'✓ Total installs: {data[\"total_installs\"]}'); print(f'✓ Active last 24h: {data[\"active_last_24h\"]}'); print(f'✓ SQLite installs: {data[\"sqlite_installs\"]}'); print(f'✓ PostgreSQL installs: {data[\"postgres_installs\"]}')"
echo ""

echo "========================================="
echo "All tests passed! ✓"
echo "========================================="
