# Tokligence Gateway

> We believe AI is becoming as essential as water and electricity. Everyone—builders first, communities next—deserves the freedom to consume and create with it, without being locked into a single provider.

Tokligence Gateway is a Golang-native, high-performance control surface for that freedom. Run it on your laptop, homelab, or favourite cloud VM and point your agents at its OpenAI-compatible endpoint. The gateway keeps your credentials organised, meters usage, and—most importantly—lets you hop between providers without touching your agent code.

Because the core is written in Go, every distribution shares the same fast, memory-efficient engine—whether you install the CLI, the managed daemon, or a Python wrapper.

> **Project status: work in progress.** Interfaces, build tooling, and docs may change without notice while we stabilise the marketplace and distribution pipelines. Early adopters are welcome, but expect rough edges.

## Own the switch, dodge the lock-in

- **Swap providers instantly:** Configure Anthropic, OpenAI, local models, or community-hosted endpoints and pivot between them with a single config change.
- **Stay self-reliant:** The gateway works on its own—no central marketplace or extra services required to start experimenting.
- **Inspect every request:** Structured logging and token accounting show exactly how each call flows and what it costs you.

## Product matrix

| Channel | Status | What ships | Ideal for | Notes |
| --- | --- | --- | --- | --- |
| Go CLI (`gateway`) | WIP | Cross-platform binaries + config templates | Builders who prefer terminals and automation | Fastest path to managing accounts, running init flows, and publishing services. |
| Go daemon (`gatewayd`) | WIP | Long-running HTTP service with usage ledger | Operators hosting shared marketplaces or teams | Same Go core, tuned for always-on workloads and observability hooks. |
| Frontend bundles (`fe/dist/web`, `fe/dist/h5`) | WIP | Optional React UI for desktop and mobile | Teams who want a visual console | Fully optional—gateway stays headless by default; consume only if you need a browser experience. |
| Python wrapper (`tokgateway`) | TODO | `pip`/`uv` wheel bundling the Go binary | Python-first users, notebooks, CI jobs | No local Go toolchain required; forwards commands to the embedded binary. |
| Docker images | TODO | Multi-arch container with CLI, daemon, configs | Kubernetes, Nomad, dev containers | Ships with both binaries; mount `config/` to customise. |

All variants are powered by the same Go kernel, so performance characteristics stay consistent across platforms.

## Looking ahead to the marketplace

We are building a lightweight marketplace that plugs into the gateway when you want it—aggregating providers across countries, clouds, and scales so supply and demand can find each other. You will be able to:

- Browse alternative token APIs, compare pricing, and route traffic where it makes sense.
- Publish your own hosted models so other gateway operators can opt in with zero code changes.
- Keep the same OpenAI-style interface while unlocking global choice.

## Configuration layers

Settings load in three tiers so you can blend predictable defaults with quick overrides:

1. `config/setting.ini` – global defaults such as `environment`, base API URLs, and logging preferences.
2. `config/<env>/gateway.ini` – environment overlays (dev/test/live) merged on top of the defaults for per-deployment tweaks.
3. Environment variables – final overrides (`PRIMARY_PROVIDER`, `GATEWAY_EMAIL`, `GATEWAY_ENABLE_PROVIDER`, etc.) for ad-hoc runs or container setups.

## Everyday commands

- `make run` – start the CLI in the foreground and watch logs stream to stdout (and optionally to a file if `log_file` is set).
- `make start` / `make stop` – background the CLI with PID management; logs default to `/tmp/model-free-gateway.log` unless configured otherwise.
- `make check` – run `scripts/smoke.sh`, which ensures the configured provider responds before you wire it into agents.
- `make test` / `make d-test` – execute the Go test suite locally or inside the bundled Docker toolchain.

## Road ahead

- Friendlier UX for swapping providers and visualising usage deltas.
- First-class flows for plugging into the forthcoming global marketplace.
- Federation hooks so neighbourhood collectives and regional co-ops can share capacity without giving up control.

Run the gateway, pick your providers, and keep the power to pivot whenever you need.

## RAG ecosystem hooks

Gateway-centric deployments can extend into Retrieval-Augmented Generation stacks without reimplementing identity or accounting:

- Use Tokligence Gateway as the shared user/auth hub while backing vector storage with systems such as Weaviate, Dgraph, or Milvus.
- Register provisioning hooks so that whenever a user or API key is created in the gateway, companion scripts can synchronise identities, ACLs, or metadata into external RAG indices.
- Keep the same Go core for rate limiting and token metering while downstream RAG services focus on embedding management and retrieval logic.

These integration points are plumbed from day one, so when we implement the dedicated RAG sync modules the surrounding product tiers will not need to refactor.
