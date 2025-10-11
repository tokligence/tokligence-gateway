# Tokligence Gateway

> We believe access to AI should be as universal as water and electricity. Every builder deserves the freedom to choose, combine, or replace LLM providers without being locked into a single vendor.

Tokligence Gateway is the open-source control surface for that freedom. Run it on your laptop, homelab, or favourite cloud VM and point your agents at its OpenAI-compatible endpoint. The gateway keeps your credentials organised, meters usage, and—most importantly—lets you hop between providers without touching your agent code.

## Own the switch, dodge the lock-in

- **Swap providers instantly:** Configure Anthropic, OpenAI, local models, or community-hosted endpoints and pivot between them with a single config change.
- **Stay self-reliant:** The gateway works on its own—no central marketplace or extra services required to start experimenting.
- **Inspect every request:** Structured logging and token accounting show exactly how each call flows and what it costs you.

## Looking ahead to the marketplace

We are building a lightweight marketplace that plugs into the gateway when you want it—aggregating providers across countries, clouds, and scales so supply and demand can find each other. You will be able to:

- Browse alternative token APIs, compare pricing, and route traffic where it makes sense.
- Publish your own hosted models so other gateway operators can opt in with zero code changes.
- Keep the same OpenAI-style interface while unlocking global choice.

Until then, the gateway already lets you curate your own roster of providers and switch freely.

## Configuration layers

Settings load in three tiers so you can blend predictable defaults with quick overrides:

1. `config/setting.ini` – global defaults such as `environment`, base API URLs, and logging preferences.
2. `config/<env>/gateway.ini` – environment overlays (dev/test/live) merged on top of the defaults for per-deployment tweaks.
3. Environment variables – final overrides (`PRIMARY_PROVIDER`, `MFG_EMAIL`, `MFG_ENABLE_PROVIDER`, etc.) for ad-hoc runs or container setups.

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
