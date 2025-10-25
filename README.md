# Tokligence Gateway

> Multi-platform LLM gateway with unified OpenAI-compatible API, supporting both standalone operation and Tokligence Token Marketplace integration.
>
> AI is becoming infrastructure. Like water and electricity, it should be accessible without vendor lock-in.
>
> Tokligence Gateway is a Golang-native, high-performance control plane that lets you switch between providers, audit their behavior, and maintain full transparency—without touching your agent code.

## Overview

Tokligence Gateway is a **platform-independent** LLM gateway that provides a unified OpenAI-compatible interface for accessing multiple model providers. The gateway prioritizes:

1. **Platform Independence**: Runs standalone on any platform (Linux, macOS, Windows) without external dependencies
2. **Flexible Deployment**: Same codebase for Community and Enterprise deployments
3. **Marketplace Integration**: Optional integration with Tokligence Token Marketplace

## Why Tokligence Gateway?

**Freedom from vendor lock-in**
Switch providers with a configuration change. No code rewrites, no migration pain.

**Privacy and control**
Keep sensitive prompts and data on your infrastructure. You decide what goes where.

**Cost optimization**
Route requests to the most cost-effective provider for each use case. Track spending in real-time.

**Reliability and failover**
Automatic fallback to alternative providers when your primary goes down. No single point of failure.

**Transparency and accountability**
Your gateway logs every token, every request, every cost. When providers make billing errors or token counting mistakes, you have the data to prove it. No more black-box charges.

**Model audit and performance tracking**
Detect when providers silently degrade service—slower responses, lower quality outputs, or throttled throughput. Your ledger creates an audit trail that reveals pattern changes over time, protecting you from stealth downgrades.

## Product Matrix

| Channel | Status | What ships | Ideal for | Notes |
| --- | --- | --- | --- | --- |
| Go CLI (`gateway`) | WIP | Cross-platform binaries + config templates | Builders who prefer terminals and automation | Command-line tool for user management, configuration, and administrative tasks. |
| Go daemon (`gatewayd`) | WIP | Long-running HTTP service with usage ledger | Operators hosting shared gateways for teams | Production-ready service with observability hooks and always-on reliability. |
| Frontend bundles (`web` and `h5`) | WIP | Optional React UI for desktop and mobile | Teams who want a visual console | Fully optional—gateway stays headless by default; enable only if you need a browser interface. |
| Python wrapper (`tokgateway`) | TODO | `pip`/`uv` wheel bundling the Go binary | Python-first users, notebooks, CI jobs | No local Go toolchain required; forwards commands to the embedded binary. |
| Docker images | TODO | Multi-arch container with CLI, daemon, configs | Kubernetes, Nomad, dev containers | Ships with both binaries; mount `config/` to customize. |

All variants are powered by the same Go codebase, ensuring consistent performance across platforms.

## Editions

| Edition | Database | Target Users | Key Features |
| --- | --- | --- | --- |
| **Community** | SQLite or PostgreSQL | Individuals and teams | Open-source core, OpenAI-compatible API, adapters, token ledger, multi-user, basic observability |
| **Enterprise** | PostgreSQL + Redis | Large organizations | Advanced routing, compliance, multi-tenancy, HA, SSO/SCIM |

**Note**: Community and Enterprise share the **same codebase**; Enterprise features are enabled via commercial license and configuration.

## Key Features

### Core Capabilities (All Editions)

#### OpenAI-Compatible API Endpoints
- ✅ **Chat Completions** (`/v1/chat/completions`)
  - Non-streaming and streaming (SSE) support
  - Full OpenAI request/response format compatibility
  - Automatic token usage tracking
- ✅ **Embeddings** (`/v1/embeddings`)
  - Single and batch text embedding
  - Support for dimensions and encoding format options
  - Compatible with OpenAI embedding models
- ✅ **Model Listing** (`/v1/models`)
  - Dynamic model discovery from all configured providers
  - Includes built-in loopback model for testing

#### Provider Adapters
- ✅ **OpenAI Adapter**
  - Chat completions (streaming & non-streaming)
  - Embeddings API
  - Full parameter support (temperature, top_p, etc.)
  - Organization header support
- ✅ **Anthropic (Claude) Adapter**
  - Format conversion from OpenAI to Anthropic API
  - Intelligent model name mapping (e.g., `claude-sonnet` → `claude-3-5-sonnet-20241022`)
  - System message handling
  - Streaming support
- ✅ **Loopback Adapter**
  - Built-in echo model for testing without external API calls
  - Deterministic responses for integration testing
  - Zero cost development and debugging

#### Intelligent Request Routing
- ✅ **Pattern-based model routing**
  - Exact match (e.g., `gpt-4`)
  - Prefix match (e.g., `*gpt-*` matches all GPT models)
  - Suffix match (e.g., `*-turbo`)
  - Contains match (e.g., `*claude*`)
- ✅ **Thread-safe concurrent routing**
- ✅ **Dynamic adapter selection** based on model name

#### Fallback & Resilience
- ✅ **Automatic failover** to alternative providers
- ✅ **Intelligent retry logic**
  - Retries on timeouts, rate limits (429), and server errors (5xx)
  - No retry on auth errors (401, 403) or not found (404)
- ✅ **Exponential backoff** with configurable retry attempts
- ✅ **Context-aware cancellation** for request cleanup

#### Token Accounting & Usage Tracking
- ✅ **Per-request ledger tracking**
  - Prompt tokens, completion tokens, total tokens
  - Service ID and model name recording
  - API key attribution
  - Consume/supply direction tracking
- ✅ **Usage summary API** per user
- ✅ **Usage logs API** with filtering and limits
- ✅ **SQLite or PostgreSQL backend** for ledger storage

#### Authentication & Access Control
- ✅ **API key management**
  - Scoped access control
  - Optional expiration dates
  - Prefix-based key identification
- ✅ **Bearer token authentication** (`Authorization: Bearer <key>`)
- ✅ **X-API-Key header support** for compatibility
- ✅ **Session-based web authentication**

#### User Management
- ✅ **Role-based access control**
  - Root admin (bypass verification)
  - Gateway admin
  - Gateway user
- ✅ **Email-based magic link authentication**
- ✅ **Bulk user import** via CSV
- ✅ **User status management** (active/suspended)

#### Platform Independence
- ✅ **Zero external dependencies** for core operation
- ✅ **Embedded SQLite** for individual deployments
- ✅ **PostgreSQL support** for production/enterprise
- ✅ **Local-only mode** when marketplace is unavailable
- ✅ **Cross-platform binaries** for Linux/macOS/Windows
- ✅ **Marketplace-optional mode** - works offline or without marketplace

#### Administrative Features
- ✅ **Bulk user import** via CSV (`gateway admin users import`)
- ✅ **API key lifecycle** management (create, list, delete)
- ✅ **Usage tracking** per API key with detailed logging
- ✅ **React web UI** (optional) for visual management
- ✅ **Command-line tools** for automation and scripting

## Quick Start

### Build from Source
```bash
# Build binaries
make build

# Output in ./bin/
# - gateway   (CLI tool)
# - gatewayd  (HTTP daemon)
```

### Configure for Local-Only Mode
```bash
# Option 1: Environment variable
export TOKLIGENCE_MARKETPLACE_ENABLED=false

# Option 2: Edit config/setting.ini
echo "marketplace_enabled=false" >> config/setting.ini
```

### Run the Gateway

#### Community (SQLite)
```bash
# Start the daemon (default: http://localhost:8081)
./bin/gatewayd

# In another terminal, start the web UI (optional)
cd fe && npm install && npm run dev
# Access at http://localhost:5174
```

#### Community (PostgreSQL)
```bash
# Set database connection
export TOKLIGENCE_IDENTITY_PATH="postgres://user:pass@localhost/tokligence"

# Start the daemon
./bin/gatewayd
```

### Initial Setup

1. **Root Admin Login** (no verification required)
   - Email: `admin@local`
   - Auto-created on first startup
   - Full administrative privileges

2. **Create Users**
   ```bash
   # Individual user
   ./bin/gateway admin users create --email user@example.com --role gateway_user
   
   # Bulk import from CSV
   ./bin/gateway admin users import --file users.csv --skip-existing
   ```

3. **Generate API Keys**
   ```bash
   ./bin/gateway admin api-keys create --user <user_id>
   ```

4. **Test the API**
   ```bash
   curl -H "Authorization: Bearer <api_key>" \
        -H "Content-Type: application/json" \
        -d '{"model":"loopback","messages":[{"role":"user","content":"Hello"}]}' \
        http://localhost:8081/v1/chat/completions
   ```

   **Note**: `loopback` is a built-in echo model that returns your input without calling real LLMs. Use it to verify authentication, configuration, and API connectivity. Zero cost, instant response.

## Configuration

Settings load in three layers:

1. **Global defaults**: `config/setting.ini`
2. **Environment overrides**: `config/{dev,test,live}/gateway.ini`
3. **Environment variables**: `TOKLIGENCE_*` prefixed variables

### Key Configuration Options

| Option | Environment Variable | Default | Description |
| --- | --- | --- | --- |
| `marketplace_enabled` | `TOKLIGENCE_MARKETPLACE_ENABLED` | `true` | Enable/disable marketplace integration |
| `admin_email` | `TOKLIGENCE_ADMIN_EMAIL` | `admin@local` | Root admin email |
| `identity_path` | `TOKLIGENCE_IDENTITY_PATH` | `~/.tokligence/identity.db` | User database (SQLite path or Postgres DSN) |
| `ledger_path` | `TOKLIGENCE_LEDGER_PATH` | `~/.tokligence/ledger.db` | Usage ledger database |
| `http_address` | - | `:8081` | HTTP server bind address |

## Architecture

### Unified Codebase
```
cmd/
├── gateway/        # CLI for admin tasks
└── gatewayd/       # HTTP daemon

internal/
├── adapter/        # Provider adapters (OpenAI, Anthropic, etc.)
├── auth/           # Authentication & sessions
├── client/         # Marketplace client (optional)
├── config/         # Configuration loading
├── core/           # Business logic
├── httpserver/     # REST API handlers
├── ledger/         # Token accounting
└── userstore/      # User/API key management
    ├── sqlite/     # Community Edition (SQLite) backend
    └── postgres/   # Community/Enterprise (PostgreSQL) backend
```

### Database Schema Compatibility
- Same schema across SQLite and PostgreSQL
- Automatic migrations on startup
- Clean upgrade path from Community to Enterprise

## Development

### Prerequisites
- Go 1.22+
- Node.js 18+ (for web UI)
- Make

### Testing
```bash
# Backend tests
make backend-test

# Frontend tests
make frontend-test

# All tests
make test
```

### Building for Distribution
```bash
# Build for all platforms
make dist

# Output in ./dist/
# ├── go/
# │   ├── linux-amd64/
# │   ├── darwin-amd64/
# │   └── windows-amd64/
# └── frontend/
#     ├── web/
#     └── h5/
```

## Tokligence Token Marketplace Integration

When `marketplace_enabled=true`:

- Browse marketplace providers and services
- Publish local models to the marketplace
- Sync usage data for billing reconciliation
- Access shared free tier quotas

The gateway gracefully degrades when the marketplace is unavailable, continuing to serve local adapters without interruption.

## Marketplace Communication & Update Checks

Tokligence Gateway maintains **optional lightweight communication** with the marketplace for two important purposes:

### 1. Version Update Notifications
The gateway periodically checks for new versions (once per 24 hours) to notify you when:
- **Critical security patches** are available
- **Important bug fixes** that affect stability
- **New features** that enhance functionality

This helps ensure your gateway stays secure and up-to-date without requiring manual monitoring.

### 2. Marketplace Promotions & Announcements
When marketplace integration is enabled, the gateway can receive:
- **Special pricing offers** from providers
- **New provider availability** in your region
- **Free tier quota updates** for marketplace users
- **Maintenance windows** to help you plan ahead

**What we send** (minimal, anonymous data):
- Installation ID (random UUID, not linked to personal info)
- Current gateway version
- Platform and architecture (e.g., "linux/amd64")
- Database type ("sqlite" or "postgres")

**What we DON'T send**:
- ❌ No personal information, emails, or IP addresses
- ❌ No prompts, responses, or API keys
- ❌ No usage patterns or request metadata
- ❌ No user count or business metrics

The installation ID is a randomly generated UUID stored in `~/.tokligence/install_id`. It cannot be traced back to you, your organization, or your hardware.

### Opt-out

If you prefer complete offline operation:

```bash
# Disable all marketplace communication
export TOKLIGENCE_MARKETPLACE_ENABLED=false

# Or disable just the update checks
export TOKLIGENCE_UPDATE_CHECK_ENABLED=false

# Add to config for persistence
echo "marketplace_enabled=false" >> config/setting.ini
echo "update_check_enabled=false" >> config/setting.ini
```

**Note**: Disabling marketplace communication does not affect core gateway functionality. All features continue to work offline with local adapters and configuration.

This communication is GDPR/CCPA compliant, completely optional, and designed to benefit users by keeping them informed of important updates and opportunities.

## Migration & Upgrades

### SQLite → PostgreSQL (Community)
```bash
# Export from SQLite
./bin/gateway migrate export --from sqlite --to postgres.sql

# Import to PostgreSQL
psql -d tokligence < postgres.sql
```

### Version Upgrades
- Database migrations run automatically on startup
- Configuration files are backward compatible
- API maintains OpenAI compatibility across versions

## Support & Documentation

- **Issues**: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- **Specifications**: See [SPEC.md](SPEC.md) for detailed technical specifications
- **Roadmap**: See [ROADMAP.md](ROADMAP.md) for development milestones
- **User Guide**: See [docs/USER_GUIDE.md](docs/USER_GUIDE.md) for setup, configuration, routing, and native endpoints

## License

- Community Edition: Apache License 2.0 — see `LICENSE` and `docs/LICENSING.md`.
- Enterprise Edition: Commercial License — contact cs@tokligence.ai or visit https://tokligence.ai.

Brand and logos are trademarks of Tokligence. See `docs/TRADEMARKS.md`.
