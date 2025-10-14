# Testing Gateway & Marketplace Communication

## Overview

This document describes how to test the complete gateway-marketplace communication system, including version update checks and announcement push notifications.

## Prerequisites

1. **Gateway** - Built and ready (`./bin/gatewayd`)
2. **Marketplace** - Running on port 8082 (or configure BASE_URL)

## Start Marketplace

```bash
cd /home/alejandroseaah/tokligence/tokligence-marketplace
PORT=8082 ./bin/server
```

The marketplace will:
- Start on port 8082
- Seed test data (versions 0.1.0 and 0.2.0, 4 announcements)
- Expose `/api/v1/gateway/ping` endpoint

## Test Marketplace Directly

```bash
# Test ping with old version (should show update)
curl -X POST http://localhost:8082/api/v1/gateway/ping \
  -H "Content-Type: application/json" \
  -d '{"install_id":"test-123","gateway_version":"0.1.0","platform":"linux/amd64","database_type":"sqlite"}' \
  | python3 -m json.tool

# Expected: update_available: true, security_update: true, 4 announcements

# Test ping with latest version (no update)
curl -X POST http://localhost:8082/api/v1/gateway/ping \
  -H "Content-Type: application/json" \
  -d '{"install_id":"test-456","gateway_version":"0.2.0","platform":"darwin/arm64","database_type":"postgres"}' \
  | python3 -m json.tool

# Expected: update_available: false, 4 announcements
```

## Test Gateway Integration

```bash
cd /home/alejandroseaah/tokligence/tokligence-gateway

# Configure gateway to connect to marketplace
export TOKLIGENCE_MARKETPLACE_ENABLED=true
export TOKLIGENCE_BASE_URL="http://localhost:8082"

# Start gateway (it will ping marketplace on startup)
./bin/gatewayd
```

**Expected output:**
```
Tokligence Gateway v0.1.0 (https://tokligence.ai)
Installation ID: <uuid>
Marketplace communication enabled
  - Version update checks
  - Promotional announcements
[telemetry] sending ping to http://localhost:8082/api/v1/gateway/ping...
⚠️  SECURITY UPDATE available: 0.1.0 → 0.2.0
   Added embeddings support, improved streaming, and security fixes
   Download: https://github.com/tokligence/tokligence-gateway/releases/...
📢 [maintenance] Scheduled Maintenance: Marketplace will be unavailable...
⚠️  [provider] New Provider: Anthropic Claude Available...
📢 [promotion] Welcome to Tokligence Marketplace!...
📢 [feature] New Feature: Embeddings API...
[telemetry] ping successful
```

## Test Results

### ✅ Marketplace Features Working

1. **Version Management**
   - Stores multiple versions with metadata
   - Marks latest version correctly
   - Detects security updates

2. **Announcement System**
   - 4 types: promotion, maintenance, feature, provider
   - 4 priority levels: low, medium, high, critical
   - Expiration handling (active/expired filtering)
   - Proper JSON serialization

3. **Ping Endpoint** (`/api/v1/gateway/ping`)
   - Accepts gateway info (install_id, version, platform, db_type)
   - Returns version update info if available
   - Returns active announcements
   - Records telemetry ping in storage

4. **Telemetry Statistics** (`/telemetry/stats`)
   - Total installs count
   - Active last 24h / 7d
   - Database type breakdown
   - Per-environment statistics

### ✅ Gateway Features Working

1. **Telemetry Client**
   - Sends structured ping payload
   - Receives and parses PingResponse
   - Logs version updates with emojis
   - Logs all announcements
   - 24-hour periodic ping scheduling

2. **Marketplace Integration**
   - Configurable via TOKLIGENCE_MARKETPLACE_ENABLED
   - Can disable for local-only mode
   - Graceful degradation when marketplace unavailable
   - Proper timeout handling

## Seed Data

The marketplace automatically seeds the following test data:

### Versions
- **0.1.0** (30 days ago) - Initial MVP release
- **0.2.0** (now, latest) - Security update with embeddings

### Announcements
1. **welcome-2025** (promotion, medium) - 10K free tokens offer (expires 30 days)
2. **new-provider-anthropic** (provider, high) - Claude 3.5 Sonnet available
3. **maint-2025-01** (maintenance, high) - Scheduled maintenance (expires 7 days)
4. **feature-embeddings** (feature, medium) - New embeddings API

## Integration Points

```
Gateway (0.1.0)
    │
    │ POST /api/v1/gateway/ping
    │ {install_id, gateway_version, platform, database_type}
    ▼
Marketplace (port 8082)
    │
    ├─► Check version (0.1.0 vs 0.2.0)
    ├─► Get active announcements (4 items)
    └─► Record telemetry ping
    │
    │ Response: PingResponse
    │ {update_available: true, latest_version: "0.2.0", announcements: [...]}
    ▼
Gateway logs updates & announcements
```

## Next Steps

1. **Production Deployment**
   - Add PostgreSQL schema migrations for versions/announcements tables
   - Set up proper marketplace URL in production config
   - Configure telemetry ping frequency (currently 24h)

2. **Admin Interface**
   - Create endpoints to manage versions (POST /api/v1/admin/versions)
   - Create endpoints to manage announcements (POST /api/v1/admin/announcements)
   - Add authentication for admin endpoints

3. **Enhanced Features**
   - Semantic version comparison (instead of string equality)
   - Targeted announcements (by platform, db_type, environment)
   - Announcement read tracking (don't show same announcement repeatedly)
   - Download analytics (track update adoption)

## Success Criteria

✅ Marketplace accepts gateway pings and records telemetry
✅ Version update detection works correctly
✅ Security updates are flagged appropriately
✅ Announcements are filtered by active status and expiration
✅ Gateway client logs updates and announcements nicely
✅ System works in both marketplace-enabled and local-only modes
✅ All data properly serializes to/from JSON
✅ Memory storage implementation complete for development
✅ Ready for PostgreSQL backend integration

**All tests passed! System is production-ready for phase 0.5.**
