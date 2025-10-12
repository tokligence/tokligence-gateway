# Developer Guide

Welcome to the Tokligence Gateway codebase. This guide helps new contributors get a local environment running, understand the repo layout, and validate changes before opening a PR.

## 1. Prerequisites

Install the following tools:

- Go ≥ 1.22 (`/usr/local/go`) for backend packages.
- Node.js ≥ 20 with `npm` for the React frontend.
- Python ≥ 3.10 (optional) for future wheel builds.
- `docker` + `docker buildx` (optional) for container images.

Ensure `$GOPATH/bin` is on your `PATH` and Go modules can download dependencies.

## 2. Repository Tour

```
cmd/
  gateway/       # CLI entrypoint
  gatewayd/      # long-running daemon
internal/
  adapter/       # upstream model connectors
  auth/          # auth helpers
  bootstrap/     # CLI init flows
  client/        # Token Exchange REST client
  config/        # INI-environment loader
  core/          # gateway orchestration logic
  hooks/         # lifecycle dispatcher + script bridge
  httpserver/    # gatewayd HTTP stack
fe/              # React + Vite frontend (web + H5)
config/          # sample configuration files
scripts/         # helper scripts (smoke tests, etc.)
```

## 3. First Run

1. Install Go dependencies: `go mod download`
2. Install frontend deps: `cd fe && npm install`
3. Initialise config: `gateway init --env dev --email you@example.com`
4. Start the CLI: `go run ./cmd/gateway`
5. Start the daemon (separate shell): `go run ./cmd/gatewayd`
6. Run the frontend in dev mode: `cd fe && npm run dev`

The CLI ensures your account exists and publishes any configured service; the daemon exposes the HTTP API on `:8081`.

## 4. Tests & Linting

- Go unit tests: `make backend-test`
- Frontend tests: `make frontend-test`
- Frontend lint: `make frontend-lint`
- Aggregate: `make test`

The hooks package includes tests that spin up helper processes—run them via `go test ./internal/hooks` if you tweak script plumbing.

## 5. Distribution Builds

For phase-0 artifacts:

- `make dist-go` — cross-compile `gateway`/`gatewayd` for Linux/macOS/Windows.
- `make dist-frontend` — build responsive web and H5 bundles under `dist/frontend/`.
- `make dist` — full matrix (Go + frontend). Clean with `make clean-dist`.

## 6. Working with Hooks

Configuration lives in `config/setting.ini` (or env vars). Example:

```
hooks_enabled=true
hooks_script_path=/usr/local/bin/sync-rag
hooks_timeout=30s
```

The CLI loads these values and registers a script handler that receives JSON events on STDIN. See `docs/hook.md` for a real script example.

## 7. Development Workflow Checklist

1. Branch from `main` and keep commits focused.
2. Run `make test` before pushing.
3. Update `README.md` when user-facing behavior changes.
4. Document new features in the `docs/` folder (do not commit unless agreed—use PR attachments or gists when necessary).
5. For distribution changes, verify `make dist-go` and `make dist-frontend` locally.

## 8. Troubleshooting Tips

- **Missing Go modules**: run `go clean -modcache` then `go mod tidy`.
- **Frontend build errors**: ensure `npm run build:web` and `npm run build:h5` succeed; Vite env variables may need refresh.
- **Hook scripts not firing**: set `TOKLIGENCE_HOOKS_ENABLED=true` and run `gateway` with `log_level=debug` to inspect CLI logs.
- **Config overrides**: environment variables take precedence over INI values.

Happy hacking! Reach out in Issues/Discussions if you hit blockers.
