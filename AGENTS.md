# Repository Guidelines

## Project Structure & Module Organization
Tokligence Gateway is split into `cmd/gateway/` (interactive CLI) and `cmd/gatewayd/` (HTTP daemon). Shared backend logic sits in `internal/` (adapters, config loaders, HTTP handlers, ledger, auth). Public helpers live under `pkg/`, while TypeScript UI code is in `fe/`. Configuration samples and model alias maps live in `config/`, and automation plus diagnostics sit in `scripts/`. End-to-end, docker, and regression suites live under `tests/`, with `tests/run_all_tests.sh` acting as the master harness.

## Build, Test, and Development Commands
- Build binaries with `make build`; target a single binary via `make bg` (CLI) or `make bgd` (daemon). For quick experiments you can still `go run ./cmd/gateway` or `go run ./cmd/gatewayd`.
- Manage the daemon through the dedicated shortcuts: `make gds`/`gdx`/`gdr` to start/stop/restart, `make gst` to inspect status, and **`make gfr` to force-restart** (kills whatever holds :8081, rotates logs, and relaunches gatewayd—useful when local runs wedge the port). Scenario-specific profiles live behind `make ansi` (Codex→Anthropic sidecar) and `make ode` (OpenAI delegation).
- Backend tests: `make be-test` (`bt`) or direct `go test ./...`. Frontend tests: `make fe-test` (`ft`). Run the full matrix with `make test`, and use `make check` to invoke `scripts/smoke.sh` for a fast HTTP sanity check.
- Docker workflows: `make docker-up-personal` / `make docker-up-team` (`dup-personal`, `dup-team`) boot the new minimal images; `make docker-build[-personal|-team]` rebuilds them, and `make dsh` opens an interactive shell.

## Coding Style & Naming Conventions
All Go code must be `gofmt`-clean; enforce with `make fmt` then `make lint`. Keep package names lower_snake_case, export types in PascalCase, and prefer table-driven tests. Frontend modules follow TypeScript + React best practices: PascalCase component files, `camelCase` hooks/utilities, and ESLint via `npm run lint`. Never hardcode API keys—reference `TOKLIGENCE_*` env vars or INI placeholders inside `config/*.ini`.

## Testing Guidelines
Place unit tests next to the code in `_test.go` or colocated `.test.tsx` files. `go test ./...` should pass without additional flags; long-running integration logic belongs in `tests/integration/` or the docker sub-suites, not in unit packages. When updating API contracts, refresh the JSON fixtures under `tests/fixtures/` and rerun `tests/run_all_tests.sh`. Frontend widgets require Vitest + @testing-library assertions and should mock network calls via the helpers in `fe/src/test-utils/`.

## Commit & Pull Request Guidelines
Use the existing `<type>: <summary>` convention (`test:`, `chore:`, `feat:`) and keep commits scoped. Do **not** append `Co-Authored-By` trailers (per `CLAUDE.md`). Every PR should explain the change, mention related issues, and attach logs or screenshots for UI or daemon behavior. Confirm `make test` (or targeted suites) succeeds before requesting review, document outward-facing changes in `README.md` or `docs/`, and ensure no generated artifacts or secrets are included. Contributions land under Apache-2.0 per `docs/LICENSING.md`.

## Configuration & Documentation Rules
Settings follow a strict three-layer hierarchy: defaults in code, environment overlays in `config/{dev,test,live}/gateway.ini`, and highest-priority `TOKLIGENCE_*` environment variables (see `CLAUDE.md`). Respect the new `TOKLIGENCE_WORK_MODE` flag—`auto`, `translation`, and `passthrough` change routing behavior globally, so note the intended mode in PR descriptions. Keep all release notes and long-form docs under `docs/` (e.g., `docs/releases/vX.Y.Z.md`), and leave the repo root for `README.md`/`README_zh.md` only; other Markdown files belong in `docs/` or subdirectories. When adding tutorial material, link back to canonical guides like `docs/QUICK_START.md`, `docs/codex-to-anthropic.md`, or `docs/claude_code-to-openai.md` instead of duplicating content.
