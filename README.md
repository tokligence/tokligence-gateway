<p align="center">
  <img src="data/images/logo_navy.png" alt="Tokligence Gateway" width="150"/>
</p>

<h1 align="center">Tokligence Gateway</h1>

<p align="center">
  <strong>High-Performance AI Gateway for Coding Agents & Enterprise Token Management</strong>
</p>

<p align="center">
  <a href="README.md">English</a> | <a href="README_zh.md">中文</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go" alt="Go Version"/>
  <img src="https://img.shields.io/badge/OS-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey" alt="Platform"/>
  <img src="https://img.shields.io/badge/Tested%20with-Codex%20CLI%20v0.55.0+-brightgreen?logo=openai" alt="Codex CLI"/>
  <img src="https://img.shields.io/badge/Tested%20with-Claude%20Code%20v2.0.29-4A90E2?logo=anthropic&logoColor=white" alt="Claude Code"/>
</p>

<p align="center">
  <a href="https://github.com/tokligence/tokligence-gateway/actions/workflows/ci.yml">
    <img src="https://github.com/tokligence/tokligence-gateway/actions/workflows/ci.yml/badge.svg" alt="CI"/>
  </a>
</p>

## 🌐 Vision

**Three pillars for the AI-native era:**

### 🛡️ The Trusted Partner for Coding Agents

Your AI agents handle sensitive code, secrets, and business data. Tokligence protects them:

- **PII Prompt Firewall** - Real-time detection and redaction of sensitive data across 100+ languages
- **API Key Protection** - Detects 30+ provider keys (OpenAI, AWS, GitHub, Stripe, etc.) before they leak to LLM providers
- **Multiple Modes** - Monitor, enforce, or redact based on your compliance needs
- **Seamless Integration** - Works with Codex CLI, Claude Code, and any OpenAI/Anthropic-compatible agent

### 🧽 The "Sponge" for SME AI Token Capacity

Think of Tokligence as a buffer for your AI throughput - smoothing capacity like a sponge absorbs water:

- **Peak Hours** - Buy tokens from the marketplace when internal LLM capacity is maxed out
- **Off-Peak** - Sell your unused LLM throughput to earn revenue
- **Elastic Scaling** - No need to over-provision; scale with actual demand

```
Traditional:     Fixed capacity ──────────────────► Waste during off-peak
Tokligence:      Demand ↗↘ ←→ Marketplace ←→ ↗↘ Supply    (elastic buffer)
```

### 🔧 Next-Gen AI Token Pipeline Infrastructure

Not just another gateway - the foundation for AI token economics:

- **Unified Access** - OpenAI, Anthropic, Gemini with bidirectional protocol translation
- **Token Ledger** - Built-in accounting and audit trail for every token consumed or sold
- **Open Source** - Apache 2.0, self-hosted, no vendor lock-in
- **High Performance** - 9.6x faster than LiteLLM with 75% less infrastructure

---

> **TL;DR**: Tokligence Gateway is a high-performance LLM gateway that protects your AI agents from data leaks, enables elastic token capacity through marketplace trading, and provides unified multi-provider access. The trusted infrastructure layer for the AI-native enterprise.

## Overview

Tokligence Gateway is a **platform-independent** LLM gateway that provides **dual native API support** - both OpenAI and Anthropic protocols - with full bidirectional translation. The gateway prioritizes:

1. **Dual Protocol Native Support**: Native OpenAI and Anthropic APIs running simultaneously with zero adapter overhead
2. **Platform Independence**: Runs standalone on any platform (Linux, macOS, Windows) without external dependencies
3. **Flexible Deployment**: Multiple installation options - pip, npm, Docker, or standalone binary
4. **Intelligent Work Modes**: Auto, passthrough, or translation modes for flexible request handling
5. **Token Trading**: Optional two-way token trading capabilities

## ⚡ Performance

Tokligence Gateway delivers exceptional performance with minimal resource footprint and industry-leading cost efficiency.

### Benchmark Results vs LiteLLM (v0.3.4, PostgreSQL)

Based on [LiteLLM's official benchmarks](https://docs.litellm.ai/docs/benchmarks) running on 4 CPU, 8GB RAM:

| Metric | LiteLLM<br/>(4 instances) | Tokligence v0.3.4<br/>(1 instance) | Improvement |
|--------|---------------------------|-----------------------------------|-------------|
| **Throughput** | 1,170 RPS | **11,227 RPS** | **9.6x faster** ✨ |
| **P50 Latency** | 100 ms | **49.66 ms** | **2x faster** ⚡ |
| **P95 Latency** | 150 ms | **78.63 ms** | **1.9x faster** 🚀 |
| **P99 Latency** | 240 ms | **93.81 ms** | **2.6x faster** |
| **Infrastructure** | 4 instances | **1 instance** | **75% reduction** 💰 |
| **Error Rate** | N/A | **0%** | Perfect stability |

**Peak Performance** (100 concurrent):
- **12,908 RPS** - absolute maximum throughput
- **P50: 7.75ms, P95: 16.47ms, P99: 21.15ms** - sub-100ms latencies
- **774,571 requests in 60 seconds** with 0% errors

**Cost Efficiency**:
- **38.4x better performance per dollar** than LiteLLM
- **1/4 infrastructure cost** (1 instance vs 4 instances)
- **9.6x higher throughput** with 75% fewer resources

See [scripts/benchmark/](scripts/benchmark/) for complete methodology, detailed results, and reproduction steps.

### Core Feature Comparison

| Feature | Tokligence Gateway | LiteLLM | OpenRouter | Cloudflare AI Gateway | AWS Bedrock |
|--------|--------------------|---------|-----------|-----------------------|-------------|
| **Protocols & translation** | Bidirectional OpenAI ↔ Anthropic with dual native APIs. | OpenAI-style in, routed out to many providers. | OpenAI-style endpoint that hides provider formats. | Normalizes OpenAI-style requests across providers. | AWS Converse API unifies Bedrock models. |
| **Work modes & routing** | Model-first auto mode chooses provider before protocol. | Flexible routing policies (cost, latency, weight). | Managed routing and fallbacks inside the SaaS gateway. | Edge-based routing with geo and A/B rules. | Routing integrated with AWS regions and services. |
| **Ports & isolation** | Single-port by default, optional multi-port isolation. | Single proxy service with config-based separation. | Single SaaS endpoint. | Cloudflare-managed edge endpoints. | Regional service endpoints managed by AWS. |
| **Clients & SDKs** | OpenAI and Anthropic SDKs (Codex, Claude Code) with no client code changes. | Great fit for Python apps using the OpenAI SDK. | Great fit for Python apps using the OpenAI SDK. | Fits apps that already terminate traffic at Cloudflare edge. | Best for AWS SDK users and Bedrock-centric stacks. |
| **Performance footprint** | Go binary with low overhead on the translation path. | Python service with more runtime overhead. | Extra network hop; latency depends on upstreams. | Runs at the edge for low global latency. | Optimized for traffic within AWS regions. |
| **Deployment & control** | Self-hosted, open-source; Docker, binary, pip, npm. | Self-hosted Python service beside your app. | Fully managed SaaS; no servers to run. | Part of the Cloudflare platform. | Managed service inside your AWS account. |
| **Ledger & audit** | Built-in token ledger for usage and audit trails. | Usage tracking available via service metrics. | Billing and usage analytics in the dashboard. | Traffic and analytics via Cloudflare tools. | Usage metrics via CloudWatch and AWS billing. |
| **Token marketplace** | Two-sided token marketplace: buy and sell unused LLM capacity. | API consumption only; no token marketplace. | API consumption only; no token marketplace. | API consumption only; no token marketplace. | API consumption only; no token marketplace. |
| **Open source** | Apache-2.0. | MIT. | Closed. | Closed. | Closed. |

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

| Channel | What ships | Ideal for | Notes |
| --- | --- | --- | --- |
| Gateway CLI (`gateway`) | Cross-platform binaries + config templates | Builders who prefer terminals and automation | Command-line tool for user management, configuration, and administrative tasks. |
| Gateway daemon (`gatewayd`) | Long-running HTTP service with usage ledger | Operators hosting shared gateways for teams | Production-ready service with observability hooks and always-on reliability. Tested with Codex CLI v0.55.0+. |
| Frontend bundles (`web` and `h5`) | Optional React UI for desktop and mobile | Teams who want a visual console | Fully optional—gateway stays headless by default; enable only if you need a browser interface. |
| Python package (`tokligence`) | `pip` package with gateway functionality | Python-first users, notebooks, CI jobs | Install via `pip install tokligence` |
| Node.js package (`@tokligence/gateway`) | `npm` package with gateway functionality | JavaScript/TypeScript developers | Install via `npm i @tokligence/gateway` |
| Docker images | Multi-arch container with CLI, daemon, configs | Kubernetes, Nomad, dev containers | Ships with both binaries; mount `config/` to customize. Available in personal and team editions. |

All variants are powered by the same Go codebase, ensuring consistent performance across platforms.

## Editions

| Edition | Database | Target Users | Key Features |
| --- | --- | --- | --- |
| **Community** | SQLite or PostgreSQL | Individuals and teams | Open-source core, OpenAI-compatible API, adapters, token ledger, multi-user, basic observability |
| **Enterprise** | PostgreSQL + Redis | Large organizations | Advanced routing, compliance, multi-tenancy, HA, SSO/SCIM |

**Note**: Community and Enterprise share the **same codebase**; Enterprise features are enabled via commercial license and configuration.

## Main Features

- **Multi-Provider Support**: OpenAI, Anthropic, and Google Gemini with unified gateway interface
- **Dual Protocol Support**: OpenAI‑compatible and Anthropic‑native APIs running simultaneously
- **Prompt Firewall**: Real-time PII detection and redaction with configurable modes (monitor, enforce, redact). Built-in regex filters + optional [Presidio sidecar](examples/firewall/presidio_sidecar/) for NLP-based detection across **100+ languages** (3-5ms latency, CPU-only). Now includes **API Key Detection** for 30+ providers (OpenAI, AWS, GitHub, Stripe, etc.)

![PII Prompt Firewall - Redact Mode](data/images/firewall_compressed.png)

*PII Prompt Firewall in Redact Mode — automatically detects and masks sensitive information.*

- **Advanced Tool Calling**: Complete OpenAI function calling with automatic Anthropic tools conversion, MCP server support, and computer use tools
- **Prompt Caching**: Request-side cache control for cost optimization with Anthropic's prompt caching
- **Code Execution**: Multi-type content blocks supporting text, images, and container uploads for code execution scenarios
- **Intelligent Duplicate Detection**: Prevents infinite loops by detecting repeated tool calls
- **Codex CLI Integration**: Full support for OpenAI Codex v0.55.0+ with Responses API and tool calling
- **Gemini Pass-through Proxy**: Native Google Gemini API support with both native and OpenAI-compatible endpoints
- **Flexible Work Modes**: Three operation modes - `auto` (smart routing), `passthrough` (delegation-only), `translation` (translation-only)
- **Multi-Port Architecture**: Default facade port 8081 with optional multi-port mode for strict endpoint isolation
- **OpenAI‑compatible chat + embeddings** (SSE and non‑SSE)
- **Anthropic‑native `/v1/messages`** with correct SSE envelope (works with Claude Code)
- **Gemini‑native `/v1beta/models/*` endpoints** with SSE streaming support
- **In‑process translation** (Anthropic ↔ OpenAI) with robust streaming and tool calling
- **Rotating logs** (daily + size), separate CLI/daemon outputs
- **Dev‑friendly auth toggle** and sensible defaults
- **Cross‑platform builds** (Linux/macOS/Windows)

Full details → see [docs/features.md](docs/features.md)

## Scenarios

- **OpenAI Codex → Anthropic Claude**: Point Codex to `http://localhost:8081/v1` (OpenAI-compatible). The gateway translates Chat Completions and Responses API requests to Anthropic, handles tool calling, and prevents infinite loops. Full support for Codex CLI v0.55.0+ including streaming, tools, and automatic duplicate detection. See [docs/codex-to-anthropic.md](docs/codex-to-anthropic.md).
- **Claude Code integration**: Point Claude Code to `http://localhost:8081/anthropic/v1/messages` (SSE). The gateway translates to OpenAI upstream and streams Anthropic‑style SSE back. Set `TOKLIGENCE_OPENAI_API_KEY` and you're ready. See [docs/claude_code-to-openai.md](docs/claude_code-to-openai.md).
- **Google Gemini integration**: Point your application to `http://localhost:8084/v1beta` for native Gemini API access or use the OpenAI-compatible endpoint at `http://localhost:8084/v1beta/openai/chat/completions`. The gateway provides pass-through proxy support for both Gemini native and OpenAI-compatible formats with SSE streaming. See [docs/gemini-integration.md](docs/gemini-integration.md).
- **Drop‑in OpenAI proxy**: Change your SDK base URL to the gateway `/v1` endpoints to get central logging, usage accounting, and routing without changing your app code.
- **Multi‑provider switching**: Route `claude*` to Anthropic, `gpt-*` to OpenAI, and `gemini-*` to Google Gemini with a config change; switch providers without touching your agent code.
- **Team gateway**: Run `gatewayd` for your team with API keys, a per‑user ledger, and small CPU/RAM footprint.
- **Local dev/offline**: Use the built‑in `loopback` model and SQLite to develop/test SSE flows without calling external LLMs.

## Quick Start & Configuration

See [docs/QUICK_START.md](docs/QUICK_START.md) for setup, configuration, logging, and developer workflow.

### Tokligence Gateway CLI Chat (`tgw chat`)

When installed via the npm package (`@tokligence/gateway`), you get an interactive CLI assistant for configuring and troubleshooting the gateway:

```bash
tgw chat
```

Example `tgw chat` session:

![Tokligence Gateway CLI Chat](data/images/tgw_chat_config_example.png)

The assistant:

- Detects available LLM endpoints (local LLMs, commercial APIs, running gateway)
- Helps you switch work modes (e.g. `auto`, `translation`, `passthrough`)
- Suggests concrete commands you can copy-paste, such as:

```bash
export TOKLIGENCE_OPENAI_API_KEY=sk-...
tgw config set work_mode translation
```

To protect secrets and comply with modern data protection practices, API keys/tokens/secrets are kept local (only masked summaries like length/prefix or `*_configured` flags are ever sent to remote LLM providers.

## Architecture


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
| `gemini_port` | Gemini-only endpoints | `:8084` |
| `facade_endpoints`, `openai_endpoints`, `anthropic_endpoints`, `gemini_endpoints`, `admin_endpoints` | Comma-separated endpoint keys per port | defaults in `internal/httpserver/server.go` |

Endpoint keys map to concrete routes:

- `openai_core`: `/v1/chat/completions`, `/v1/embeddings`, `/v1/models`
- `openai_responses`: `/v1/responses`
- `anthropic`: `/anthropic/v1/messages`, `/v1/messages`, and their `count_tokens` variants
- `gemini_native`: `/v1beta/models/*` (native Gemini API and OpenAI-compatible endpoints)
- `admin`: `/api/v1/admin/...`
- `health`: `/health`

Example configuration enabling multi-port mode:

```ini
multiport_mode = true
facade_port = :8081
admin_port = :8079
openai_port = :8082
anthropic_port = :8083
gemini_port = :8084

openai_endpoints = openai_core,openai_responses,health
anthropic_endpoints = anthropic,health
gemini_endpoints = gemini_native,health
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
| `POST /v1beta/models/{model}:generateContent` | Gemini | Native Gemini chat completion | Google AI SDK |
| `POST /v1beta/models/{model}:streamGenerateContent` | Gemini | Native Gemini streaming | Google AI SDK |
| `POST /v1beta/models/{model}:countTokens` | Gemini | Token counting | Google AI SDK |
| `GET /v1beta/models` | Gemini | List Gemini models | Google AI SDK |
| `POST /v1beta/openai/chat/completions` | Gemini (OpenAI-compatible) | OpenAI-format on Gemini | OpenAI SDK with Gemini models |

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

## Token Trading Network (optional)

When enabled, you can connect to the Tokligence token trading network to buy and sell token capacity. The gateway works fully offline by default.

## Updates & Minimal Telemetry

Optional daily update check sends only non‑PII basics (random install ID, version, platform/db). Disable with `TOKLIGENCE_UPDATE_CHECK_ENABLED=false`. Core functionality works fully offline.

## Compatibility

- **OpenAI Codex CLI v0.55.0+**: Fully compatible with Codex CLI using Responses API. Supports streaming, tool calling, automatic shell command normalization, and duplicate detection to prevent infinite loops.
- **Claude Code v2.0.29**: Verified end‑to‑end with Anthropic `/v1/messages` over SSE. The gateway translates Anthropic requests to OpenAI as needed and streams Anthropic‑style SSE back to the client.

### ✅ Verified with Claude Code

Claude Code pointing at `http://localhost:8081/anthropic` (dummy API key, OpenAI key configured on gateway) talking to GPT via translation:

![Claude Code to GPT via Gateway](data/images/claude-to-gpt.png)

### Auto Mode: Model First, Endpoint Second

In `work_mode=auto`, the gateway first infers the provider from the requested `model` (via `model_provider_routes`, e.g., `gpt*→openai`, `claude*→anthropic`). That choice overrides endpoint hints; the endpoint (`/v1/messages`, `/v1/chat/completions`, `/v1/responses`) only decides whether to translate or passthrough once the provider is known. Add vendor prefixes you trust (e.g., `o1*→openai`, `qwen*→ali`) via config rather than relying on broad wildcards. If the inferred provider is unavailable, the gateway translates via the other provider using the configured defaults.

When `TOKLIGENCE_CHAT_TO_ANTHROPIC=on` (or `chat_to_anthropic=true` in `gateway.ini`), this model-first policy also applies to the OpenAI Chat endpoint: `/v1/chat/completions` with a `claude*` model is translated to Anthropic `/v1/messages` (non‑streaming returns Anthropic JSON, streaming maps SSE back into OpenAI `chat.completion.chunk` events), while `gpt*` models continue to use native OpenAI Chat.

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
   - Google Gemini Integration: [docs/gemini-integration.md](docs/gemini-integration.md)
   - Prompt Firewall: [docs/PROMPT_FIREWALL.md](docs/PROMPT_FIREWALL.md) | [Quick Start](examples/firewall/README.md)

## License

- Community Edition: Apache License 2.0 — see `LICENSE` and `docs/LICENSING.md`.
- Enterprise Edition: Commercial License — contact cs@tokligence.ai or visit https://tokligence.ai.

Brand and logos are trademarks of Tokligence. See `docs/TRADEMARKS.md`.
