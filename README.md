# Tokligence Gateway

![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/OS-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)
![Claude Code](https://img.shields.io/badge/Tested%20with-Claude%20Code%20v2.0.29-4A90E2)

> AI is becoming infrastructure. Like water and electricity, it should be accessible without vendor lock-in.
>
> Tokligence Gateway is a Golang-native, high-performance control plane that lets you switch between providers, audit their behavior, and maintain full transparency—without touching your agent code.
>
> Multi-platform LLM gateway with unified OpenAI-compatible API, supporting both standalone operation and Tokligence Token Marketplace integration.

## Overview

Tokligence Gateway is a **platform-independent** LLM gateway that provides a unified OpenAI-compatible interface for accessing multiple model providers. The gateway prioritizes:

1. **Platform Independence**: Runs standalone on any platform (Linux, macOS, Windows) without external dependencies
2. **Flexible Deployment**: Same codebase for Community and Enterprise deployments
3. **Marketplace Integration**: Optional integration with Tokligence Token Marketplace

## Requirements

- Go 1.24 or newer
- Make (optional, for convenience targets)
- Node.js 18+ (only if you build the optional frontend)

## Installation

Tokligence Gateway is now available on multiple platforms via package managers:

### Python (pip)
```bash
pip install tokligence
```

### Node.js (npm)
```bash
npm i @tokligence/gateway
```

### From Source
```bash
git clone https://github.com/tokligence/tokligence-gateway
cd tokligence-gateway
make build
```

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
| Python package (`tokligence`) | **Available** | `pip` package with gateway functionality | Python-first users, notebooks, CI jobs | Install via `pip install tokligence` |
| Node.js package (`@tokligence/gateway`) | **Available** | `npm` package with gateway functionality | JavaScript/TypeScript developers | Install via `npm i @tokligence/gateway` |
| Docker images | TODO | Multi-arch container with CLI, daemon, configs | Kubernetes, Nomad, dev containers | Ships with both binaries; mount `config/` to customize. |

All variants are powered by the same Go codebase, ensuring consistent performance across platforms.

## Editions

| Edition | Database | Target Users | Key Features |
| --- | --- | --- | --- |
| **Community** | SQLite or PostgreSQL | Individuals and teams | Open-source core, OpenAI-compatible API, adapters, token ledger, multi-user, basic observability |
| **Enterprise** | PostgreSQL + Redis | Large organizations | Advanced routing, compliance, multi-tenancy, HA, SSO/SCIM |

**Note**: Community and Enterprise share the **same codebase**; Enterprise features are enabled via commercial license and configuration.

## Main Features

- OpenAI‑compatible chat + embeddings (SSE and non‑SSE)
- Anthropic‑native `/v1/messages` with correct SSE envelope (works with Claude Code)
- In‑process translation (Anthropic ↔ OpenAI) with robust streaming
- Rotating logs (daily + size), separate CLI/daemon outputs
- Dev‑friendly auth toggle and sensible defaults
- Cross‑platform builds (Linux/macOS/Windows)

Full details → see [docs/features.md](docs/features.md)

## Scenarios

- Claude Code integration: point Claude Code to `http://localhost:8081/anthropic/v1/messages` (SSE). The gateway translates to OpenAI upstream and streams Anthropic‑style SSE back. Set `TOKLIGENCE_OPENAI_API_KEY` and you’re ready.
- Drop‑in OpenAI proxy: change your SDK base URL to the gateway `/v1` endpoints to get central logging, usage accounting, and routing without changing your app code.
- Multi‑provider switching: route `claude*` to Anthropic and `gpt-*` to OpenAI with a config change; switch providers without touching your agent code.
- Team gateway: run `gatewayd` for your team with API keys, a per‑user ledger, and small CPU/RAM footprint.
- Local dev/offline: use the built‑in `loopback` model and SQLite to develop/test SSE flows without calling external LLMs.

## Quick Start & Configuration

See [docs/QUICK_START.md](docs/QUICK_START.md) for setup, configuration, logging, and developer workflow.

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

- Requirements: Go 1.24+, Node 18+ (if building the optional frontend), Make.
- For local workflow (build, run, scripts), see docs/QUICK_START.md.

## Tokligence Token Marketplace (optional)

When enabled, you can browse providers/services and sync usage for billing. The gateway works fully offline (or without marketplace) by default.

## Quick Start & Configuration

See docs/QUICK_START.md for setup, configuration, logging, and developer workflow.

## Updates & Minimal Telemetry

Optional daily update check sends only non‑PII basics (random install ID, version, platform/db). Disable with `TOKLIGENCE_UPDATE_CHECK_ENABLED=false`. Core functionality works fully offline.

## Compatibility

- Verified end‑to‑end with Claude Code v2.0.29 (Anthropic `/v1/messages` over SSE). The gateway translates Anthropic requests to OpenAI as needed and streams Anthropic‑style SSE back to the client.

 

## Support & Documentation

- Issues: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- Full features: [docs/features.md](docs/features.md)
- Release notes: [docs/releases/](docs/releases/)
- Changelog: [CHANGELOG.md](CHANGELOG.md)

## License

- Community Edition: Apache License 2.0 — see `LICENSE` and `docs/LICENSING.md`.
- Enterprise Edition: Commercial License — contact cs@tokligence.ai or visit https://tokligence.ai.

Brand and logos are trademarks of Tokligence. See `docs/TRADEMARKS.md`.
