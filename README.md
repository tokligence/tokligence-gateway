# Tokligence Gateway

**Language**: English | [中文](README_zh.md)

![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/OS-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)
![Codex CLI](https://img.shields.io/badge/Tested%20with-Codex%20CLI%20v0.55.0+-brightgreen?logo=openai)
![Claude Code](https://img.shields.io/badge/Tested%20with-Claude%20Code%20v2.0.29-4A90E2?logo=anthropic&logoColor=white)

## 🌐 Vision: The First Decentralized AI Compute Marketplace

**We're not building just another LLM gateway. We're creating the world's first two-way AI compute marketplace.**

### Why This Matters

AI is becoming as essential as water and electricity. But unlike these utilities, AI compute is controlled by a few tech giants. We believe:

- 🔌 **AI should be infrastructure** - Accessible to all, monopolized by none
- 🔄 **Every consumer can be a provider** - Your idle GPU can serve others, like Bitcoin mining democratized finance
- 🌍 **The future is distributed** - Inference and training GPUs will separate, creating a global compute mesh

### The Game-Changing Difference: Two-Way Marketplace

```
Traditional Gateways:  User → Gateway → Provider  (one-way consumption)
Tokligence:           User ↔ Gateway ↔ Marketplace (buy AND sell)
```

With Tokligence, every installation becomes a node in a global AI compute network. You can:
- **Buy** tokens when you need AI capacity
- **Sell** unused GPU cycles back to the network
- **Arbitrage** between different prices and availability

**Our bet**: The future of AI isn't centralized providers, but a mesh network where every GPU owner can sell capacity and every developer can access the global pool.

---

> **TL;DR**: Tokligence Gateway is a Golang-native, high-performance LLM gateway that not only provides unified access to multiple AI providers but also enables you to sell your unused compute back to the network. Think of it as Airbnb for AI compute.

## Overview

Tokligence Gateway is a **platform-independent** LLM gateway that provides **dual native API support** - both OpenAI and Anthropic protocols - with full bidirectional translation. The gateway prioritizes:

1. **Dual Protocol Native Support**: Native OpenAI and Anthropic APIs running simultaneously with zero adapter overhead
2. **Platform Independence**: Runs standalone on any platform (Linux, macOS, Windows) without external dependencies
3. **Flexible Deployment**: Multiple installation options - pip, npm, Docker, or standalone binary
4. **Intelligent Work Modes**: Auto, passthrough, or translation modes for flexible request handling
5. **Marketplace Integration**: Optional integration with Tokligence Token Marketplace

### Core Architecture Comparison

| Feature | Tokligence Gateway | LiteLLM | OpenRouter | Cloudflare AI Gateway | AWS Bedrock |
|---------|-------------------|---------|------------|---------------------|-------------|
| **🔄 Bidirectional API Translation** | ✅ **Full bidirectional**<br/>• OpenAI ↔ Anthropic translation<br/>• Messages, tools, streaming<br/>• Zero code change for clients<br/>• Automatic protocol adaptation | ❌ One-way only<br/>OpenAI format input<br/>Provider-specific output<br/>No reverse translation | ⚠️ Unclear<br/>OpenAI-compatible input<br/>May have internal translation<br/>Closed source | ❌ One-way only<br/>OpenAI-compatible input<br/>Limited protocol support | ❌ One-way only<br/>Proprietary Converse API<br/>AWS-specific format |
| **🌐 Two-Way Marketplace** | ✅ **World's first**<br/>Buy AND sell compute<br/>True two-way economy | ❌ Consume only | ❌ Consume only | ❌ Consume only | ❌ Consume only |
| **🛠️ Advanced Tool Calling** | ✅ **Cross-protocol intelligence**<br/>• Tool format auto-translation<br/>• Smart filtering (apply_patch, etc.)<br/>• Infinite loop detection<br/>• Session state management | ⚠️ Basic pass-through<br/>OpenAI format only<br/>No cross-protocol support<br/>No loop detection | ✅ Good support<br/>Parallel tool calls<br/>Interleaved thinking<br/>OpenAI format only | ⚠️ Workers AI only<br/>Not via REST API<br/>Embedded execution only | ✅ Good support<br/>Converse API<br/>Fine-grained streaming<br/>AWS models only |
| **🔌 Deployment** | ✅ **Maximum flexibility**<br/>Pip, npm, Docker, Binary<br/>Self-hosted or cloud<br/>Zero dependencies<br/>Any platform | ⚠️ Python environment<br/>SDK + Proxy mode<br/>Pip install required | ☁️ SaaS only<br/>No self-host option<br/>Vendor lock-in | ☁️ Cloudflare bound<br/>Platform dependency<br/>Edge network only | ☁️ AWS bound<br/>Regional deployment<br/>AWS ecosystem only |
| **💾 Data Sovereignty** | ✅ **Complete control**<br/>100% local deployment<br/>SQLite/PostgreSQL<br/>Your infrastructure | ✅ Good<br/>Self-hosted option<br/>Full data control | ⚠️ Limited<br/>Zero logging by default<br/>Data flows through proxy<br/>Opt-in logging for discount | ⚠️ Limited<br/>Cloudflare edge nodes<br/>Managed service model | ⚠️ Limited<br/>AWS infrastructure<br/>Region-specific<br/>AWS security model |
| **📊 Cost Tracking & Audit** | ✅ **Forensic-level precision**<br/>Token-level ledger<br/>Historical pricing tracking<br/>Provider billing verification<br/>Multi-provider audit trail | ✅ Good<br/>Automatic spend tracking<br/>Per-model costs<br/>Requires base_model config | ✅ **Excellent**<br/>Transparent per-token billing<br/>No markup on inference<br/>5% fee on credit purchase<br/>Provider-accurate | ✅ Good<br/>Unified billing<br/>Cross-provider analytics<br/>Cost monitoring | ⚠️ Basic<br/>CloudWatch metrics<br/>AWS billing integration<br/>AWS pricing model |
| **🚀 Performance** | ✅ **Native speed**<br/>Go compiled binary<br/>Sub-millisecond overhead<br/>Minimal memory footprint | ⚠️ Python overhead<br/>Higher memory usage<br/>P99 latency improved in 2025 | ⚠️ Variable<br/>Proxy latency overhead<br/>Provider-dependent<br/>Global routing | ✅ Excellent<br/>Edge acceleration<br/>Up to 90% latency reduction<br/>Global CDN | ✅ Good<br/>Regional endpoints<br/>Low latency in AWS regions |
| **🔓 Open Source** | ✅ **Fully open**<br/>Apache 2.0<br/>Complete source code<br/>GitHub available | ✅ Open<br/>MIT License<br/>GitHub: BerriAI/litellm | ❌ Closed source<br/>Proprietary SaaS | ❌ Closed source<br/>Managed service | ❌ Closed source<br/>AWS proprietary |

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
| Gateway CLI (`gateway`) | v0.3.0 | Cross-platform binaries + config templates | Builders who prefer terminals and automation | Command-line tool for user management, configuration, and administrative tasks. |
| Gateway daemon (`gatewayd`) | v0.3.0 | Long-running HTTP service with usage ledger | Operators hosting shared gateways for teams | Production-ready service with observability hooks and always-on reliability. Tested with Codex CLI v0.55.0+. |
| Frontend bundles (`web` and `h5`) | v0.3.0 | Optional React UI for desktop and mobile | Teams who want a visual console | Fully optional—gateway stays headless by default; enable only if you need a browser interface. |
| Python package (`tokligence`) | v0.3.0 | `pip` package with gateway functionality | Python-first users, notebooks, CI jobs | Install via `pip install tokligence` |
| Node.js package (`@tokligence/gateway`) | v0.3.0 | `npm` package with gateway functionality | JavaScript/TypeScript developers | Install via `npm i @tokligence/gateway` |
| Docker images | v0.3.0 | Multi-arch container with CLI, daemon, configs | Kubernetes, Nomad, dev containers | Ships with both binaries; mount `config/` to customize. Available in personal and team editions. |

All variants are powered by the same Go codebase, ensuring consistent performance across platforms.

## Editions

| Edition | Database | Target Users | Key Features |
| --- | --- | --- | --- |
| **Community** | SQLite or PostgreSQL | Individuals and teams | Open-source core, OpenAI-compatible API, adapters, token ledger, multi-user, basic observability |
| **Enterprise** | PostgreSQL + Redis | Large organizations | Advanced routing, compliance, multi-tenancy, HA, SSO/SCIM |

**Note**: Community and Enterprise share the **same codebase**; Enterprise features are enabled via commercial license and configuration.

## Main Features

- **Dual Protocol Support**: OpenAI‑compatible and Anthropic‑native APIs running simultaneously
- **Full Tool Calling Support**: Complete OpenAI function calling with automatic Anthropic tools conversion
- **Intelligent Duplicate Detection**: Prevents infinite loops by detecting repeated tool calls
- **Codex CLI Integration**: Full support for OpenAI Codex v0.55.0+ with Responses API and tool calling
- **Flexible Work Modes**: Three operation modes - `auto` (smart routing), `passthrough` (delegation-only), `translation` (translation-only)
- **Multi-Port Architecture**: Default facade port 8081 with optional multi-port mode for strict endpoint isolation
- **OpenAI‑compatible chat + embeddings** (SSE and non‑SSE)
- **Anthropic‑native `/v1/messages`** with correct SSE envelope (works with Claude Code)
- **In‑process translation** (Anthropic ↔ OpenAI) with robust streaming and tool calling
- **Rotating logs** (daily + size), separate CLI/daemon outputs
- **Dev‑friendly auth toggle** and sensible defaults
- **Cross‑platform builds** (Linux/macOS/Windows)

Full details → see [docs/features.md](docs/features.md)

## Scenarios

- **OpenAI Codex → Anthropic Claude**: Point Codex to `http://localhost:8081/v1` (OpenAI-compatible). The gateway translates Chat Completions and Responses API requests to Anthropic, handles tool calling, and prevents infinite loops. Full support for Codex CLI v0.55.0+ including streaming, tools, and automatic duplicate detection. See [docs/codex-to-anthropic.md](docs/codex-to-anthropic.md).
- **Claude Code integration**: Point Claude Code to `http://localhost:8081/anthropic/v1/messages` (SSE). The gateway translates to OpenAI upstream and streams Anthropic‑style SSE back. Set `TOKLIGENCE_OPENAI_API_KEY` and you're ready. See [docs/claude_code-to-openai.md](docs/claude_code-to-openai.md).
- **Drop‑in OpenAI proxy**: Change your SDK base URL to the gateway `/v1` endpoints to get central logging, usage accounting, and routing without changing your app code.
- **Multi‑provider switching**: Route `claude*` to Anthropic and `gpt-*` to OpenAI with a config change; switch providers without touching your agent code.
- **Team gateway**: Run `gatewayd` for your team with API keys, a per‑user ledger, and small CPU/RAM footprint.
- **Local dev/offline**: Use the built‑in `loopback` model and SQLite to develop/test SSE flows without calling external LLMs.

## Quick Start & Configuration

See [docs/QUICK_START.md](docs/QUICK_START.md) for setup, configuration, logging, and developer workflow.

## Architecture

### Project Structure
```
cmd/
├── gateway/        # CLI for admin tasks and configuration
└── gatewayd/       # HTTP daemon (long-running service)

internal/
├── adapter/        # Provider adapters (OpenAI, Anthropic, loopback, router)
│   ├── anthropic/  # Anthropic API client
│   ├── openai/     # OpenAI API client
│   ├── loopback/   # Testing adapter
│   ├── fallback/   # Fallback handling
│   └── router/     # Model-based routing
├── httpserver/     # HTTP server and endpoint handlers
│   ├── anthropic/  # Anthropic protocol handlers
│   ├── openai/     # OpenAI protocol handlers
│   ├── responses/  # Responses API session management
│   ├── tool_adapter/ # Tool filtering and adaptation
│   ├── endpoints/  # Endpoint registration
│   └── protocol/   # Protocol definitions
├── translation/    # Anthropic ↔ OpenAI protocol translation
│   ├── adapter/    # Translation logic
│   └── adapterhttp/ # HTTP handler for sidecar mode
├── sidecar/        # Sidecar mode adapters (Claude Code → OpenAI)
├── auth/           # Authentication & API key validation
├── userstore/      # User and API key management
│   ├── sqlite/     # SQLite backend (Community)
│   └── postgres/   # PostgreSQL backend (Community/Enterprise)
├── ledger/         # Token accounting and usage tracking
│   └── sqlite/     # SQLite ledger storage
├── config/         # Configuration loading (INI + env)
├── core/           # Business logic and domain models
├── openai/         # OpenAI type definitions
├── bridge/         # SSE bridge adapters
├── client/         # Marketplace client (optional)
├── hooks/          # Lifecycle hook dispatchers
├── logging/        # Structured logging
├── telemetry/      # Metrics and monitoring
├── bootstrap/      # Application initialization
├── contracts/      # Interface contracts
└── testutil/       # Testing utilities
```

### Dual Protocol Architecture

The gateway exposes **both OpenAI and Anthropic API formats** simultaneously, with intelligent routing based on your configuration:

```
┌──────────────────────────────────────────┐
│           Clients                        │
│  ─────────────────────────────────       │
│  • OpenAI SDK / Codex                    │
│  • Claude Code                           │
│  • LangChain / Any compatible tool       │
└──────────────────────────────────────────┘
                    ▼
┌──────────────────────────────────────────┐
│   Tokligence Gateway (Facade :8081)      │
│  ─────────────────────────────────       │
│                                          │
│  OpenAI-Compatible API:                 │
│    POST /v1/chat/completions             │
│    POST /v1/responses                    │
│    GET  /v1/models                       │
│    POST /v1/embeddings                   │
│                                          │
│  Anthropic Native API:                   │
│    POST /anthropic/v1/messages           │
│    POST /anthropic/v1/messages/count_tokens│
│                                          │
│  Work Mode: auto | passthrough | translation │
└──────────────────────────────────────────┘
                    ▼
        ┌───────────────────────┐
        │   Router Adapter      │
        │  (Model-based routing)│
        └───────────────────────┘
             ▼           ▼
    ┌──────────┐   ┌──────────┐
    │  OpenAI  │   │Anthropic │
    │  Adapter │   │  Adapter │
    └──────────┘   └──────────┘
         ▼              ▼
  ┌──────────┐   ┌──────────┐
 │ OpenAI   │   │Anthropic │
 │   API    │   │   API    │
 └──────────┘   └──────────┘
```

### Multi-Port Architecture

**Default Mode (Single-Port)**: The gateway runs on facade port **:8081** by default, exposing all endpoints (OpenAI, Anthropic, admin) on a single port for simplicity.

**Multi-Port Mode (Optional)**: Enable `multiport_mode=true` for strict endpoint isolation across dedicated ports:

| Config key | Description | Default |
| --- | --- | --- |
| `multiport_mode` | Enable multi-port mode | `false` |
| `facade_port` | Main aggregator listener (all endpoints) | `:8081` |
| `admin_port` | Admin-only endpoints | `:8079` |
| `openai_port` | OpenAI-only endpoints | `:8082` |
| `anthropic_port` | Anthropic-only endpoints | `:8083` |
| `facade_endpoints`, `openai_endpoints`, `anthropic_endpoints`, `admin_endpoints` | Comma-separated endpoint keys per port | defaults in `internal/httpserver/server.go` |

Endpoint keys map to concrete routes:

- `openai_core`: `/v1/chat/completions`, `/v1/embeddings`, `/v1/models`
- `openai_responses`: `/v1/responses`
- `anthropic`: `/anthropic/v1/messages`, `/v1/messages`, and their `count_tokens` variants
- `admin`: `/api/v1/admin/...`
- `health`: `/health`

Example configuration enabling multi-port mode:

```ini
multiport_mode = true
facade_port = :8081
admin_port = :8079
openai_port = :8082
anthropic_port = :8083

openai_endpoints = openai_core,openai_responses,health
anthropic_endpoints = anthropic,health
admin_endpoints = admin,health
```

The regression suite (`go test ./...` and `tests/run_all_tests.sh`) now exercises `/v1/responses` streaming on every listener to ensure the bridge produces the expected SSE sequence across ports.

### API Endpoints

| Endpoint | Protocol | Purpose | Example Client |
|----------|----------|---------|----------------|
| `POST /v1/chat/completions` | OpenAI | Chat with tool calling support | OpenAI SDK, LangChain |
| `POST /v1/responses` | OpenAI | Responses API with session management | **Codex CLI v0.55.0+** |
| `GET /v1/models` | OpenAI | List available models | Any OpenAI client |
| `POST /v1/embeddings` | OpenAI | Text embeddings | LangChain, OpenAI SDK |
| `POST /anthropic/v1/messages` | Anthropic | Native Anthropic chat | Claude Code |
| `POST /anthropic/v1/messages/count_tokens` | Anthropic | Token estimation | Claude Code |

### Routing Mechanism

The gateway routes requests based on **model name patterns**:

```bash
# Configuration via environment variable
TOKLIGENCE_ROUTES=claude*=>anthropic,gpt-*=>openai

# Examples:
model: "claude-3-haiku"     → Anthropic API
model: "claude-3.5-sonnet"  → Anthropic API
model: "gpt-4"              → OpenAI API
model: "gpt-3.5-turbo"      → OpenAI API
```

### Work Modes

The gateway supports three work modes for flexible request handling:

| Mode | Behavior | Use Case |
|------|----------|----------|
| **`auto`** (default) | Smart routing - automatically chooses passthrough or translation based on endpoint+model match | Best for mixed workloads; `/v1/responses` + gpt* = passthrough, `/v1/responses` + claude* = translation |
| **`passthrough`** | Delegation-only - direct passthrough to upstream providers, rejects translation requests | Force all requests to be delegated to native providers without translation |
| **`translation`** | Translation-only - only allows translation between API formats, rejects passthrough requests | Force all requests through the translation layer for testing or protocol conversion |

```bash
# Configuration via environment variable or INI
TOKLIGENCE_WORK_MODE=auto|passthrough|translation

# Or in config/dev/gateway.ini
work_mode=auto
```

**Examples**:
- `work_mode=auto`: `/v1/responses` with `gpt-4` → delegates to OpenAI; with `claude-3.5-sonnet` → translates to Anthropic
- `work_mode=passthrough`: Only allows native provider delegation (e.g., gpt* to OpenAI, claude* to Anthropic via their native APIs)
- `work_mode=translation`: Only allows cross-protocol translation (e.g., Codex → Anthropic via OpenAI Responses API translation)

### Key Features

1. **Protocol Transparency**: Clients choose their preferred API format (OpenAI or Anthropic)
2. **Flexible Routing**: Configuration-driven backend selection without code changes
3. **Automatic Format Conversion**: Seamless OpenAI ↔ Anthropic translation
4. **Tool Calling Support**: Full OpenAI function calling with Anthropic tools conversion
5. **Unified Logging**: All requests logged to a single ledger database

### Database Schema Compatibility
- Same schema across SQLite and PostgreSQL
- Automatic migrations on startup
- Clean upgrade path from Community to Enterprise

## Development

- Requirements: Go 1.24+, Node 18+ (if building the optional frontend), Make.
- For local workflow (build, run, scripts), see [docs/QUICK_START.md](docs/QUICK_START.md).

## Tokligence Token Marketplace (optional)

When enabled, you can browse providers/services and sync usage for billing. The gateway works fully offline (or without marketplace) by default.

## Updates & Minimal Telemetry

Optional daily update check sends only non‑PII basics (random install ID, version, platform/db). Disable with `TOKLIGENCE_UPDATE_CHECK_ENABLED=false`. Core functionality works fully offline.

## Compatibility

- **OpenAI Codex CLI v0.55.0+**: Fully compatible with Codex CLI using Responses API. Supports streaming, tool calling, automatic shell command normalization, and duplicate detection to prevent infinite loops.
- **Claude Code v2.0.29**: Verified end‑to‑end with Anthropic `/v1/messages` over SSE. The gateway translates Anthropic requests to OpenAI as needed and streams Anthropic‑style SSE back to the client.

### ✅ Verified with Codex CLI

The gateway has been tested and verified with OpenAI Codex CLI in full-auto mode:

**Test Command:**
```bash
codex --full-auto --config 'model="claude-3-5-sonnet-20241022"'
```

**Configuration:**
- Base URL pointed to gateway: `http://localhost:8081/v1`
- Model: `claude-3-5-sonnet-20241022` (Anthropic Claude)
- Mode: Full-auto with tool calling enabled
- API: OpenAI Responses API with streaming

**Screenshot:**

![Codex CLI with Gateway](data/images/codex-to-anthropic.png)

The test demonstrates:
- ✅ Seamless Codex → Gateway → Anthropic flow
- ✅ Tool calling (shell commands) working correctly
- ✅ Streaming responses in real-time
- ✅ Duplicate detection preventing infinite loops
- ✅ Automatic shell command normalization

For detailed setup instructions, see [docs/codex-to-anthropic.md](docs/codex-to-anthropic.md).



## Support & Documentation

- Issues: [GitHub Issues](https://github.com/tokligence/tokligence-gateway/issues)
- Full features: [docs/features.md](docs/features.md)
- Release notes: [docs/releases/](docs/releases/)
- Changelog: [docs/CHANGELOG.md](docs/CHANGELOG.md)
- Integration guides:
   - Codex → Anthropic via Gateway: [docs/codex-to-anthropic.md](docs/codex-to-anthropic.md)
   - Claude Code → OpenAI via Gateway: [docs/claude_code-to-openai.md](docs/claude_code-to-openai.md)

## License

- Community Edition: Apache License 2.0 — see `LICENSE` and `docs/LICENSING.md`.
- Enterprise Edition: Commercial License — contact cs@tokligence.ai or visit https://tokligence.ai.

Brand and logos are trademarks of Tokligence. See `docs/TRADEMARKS.md`.
