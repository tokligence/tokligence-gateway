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
- ✅ **OpenAI-compatible API** (`/v1/chat/completions`, `/v1/models`, `/v1/embeddings`)
- ✅ **Multi-provider support** via adapter architecture
- ✅ **Token accounting** with per-request ledger tracking
- ✅ **API key management** with scoped access control
- ✅ **User management** (root admin, operators, consumers)
- ✅ **Marketplace-optional mode** - works offline or without marketplace

### Platform Independence
- **Zero external dependencies** for core operation
- **Embedded SQLite** for individual deployments
- **Local-only mode** when marketplace is unavailable
- **Cross-platform binaries** for Linux/macOS/Windows

### Administrative Features
- **Bulk user import** via CSV (`gateway admin users import`)
- **API key lifecycle** management
- **Usage tracking** per API key
- **React web UI** (optional) for visual management

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

## License

- Community Edition: Apache License 2.0 — see `LICENSE` and `docs/LICENSING.md`.
- Enterprise Edition: Commercial License — contact cs@tokligence.ai or visit https://tokligence.ai.

Brand and logos are trademarks of Tokligence. See `docs/TRADEMARKS.md`.
