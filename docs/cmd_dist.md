# Command Distribution Support

## Distribution Objectives
- Keep the Go-based gateway (CLI + gatewayd) canonical; every other channel wraps the same binaries.
- Offer first-class installs for Python users via `pip`/`uv` without requiring Go or Node toolchains.
- Ship prebuilt artifacts for Linux, macOS, and Windows, plus browser/H5 frontends that talk to the gatewayd API.
- Keep configuration discoverable: defaults live in `config/`, can be overridden via CLI flags or environment variables.

## Artifact Matrix
| Channel | Format | Primary Consumers | Notes |
| --- | --- | --- | --- |
| Go CLI (`gateway`) | `tar.gz`/`zip` per GOOS-GOARCH | DevOps, self-hosters | Built via Go toolchain or GoReleaser; includes `config/` templates.
| Go daemon (`gatewayd`) | `tar.gz`/`zip` + optional systemd unit | Gateway operators | Shares build pipeline with CLI; optionally installable as service.
| Python wrapper (`tokgateway`) | Wheel + source distribution | Python/`uv` users | Wheel bundles the matching Go binary; platform-specific wheels only.
| Docker images | `ghcr.io/tokligence/gateway:<tag>` | Container users | Single image with CLI + daemon + static assets.
| Web console | `fe/dist/web` static bundle | Desktop browsers | Generated with Vite; served by CDN or behind gatewayd.
| Mobile H5 console | `fe/dist/h5` static bundle | Mobile browsers | Same codebase with mobile-oriented theme/viewport overrides.

All build outputs land under `dist/` and follow this layout:

```
dist/
  go/
    gateway-linux-amd64/
    gateway-darwin-arm64/
    ...
  python/
    wheels/
    sdist/
  docker/
    gateway.tar
  frontend/
    web/
    h5/
```

## Build Prerequisites
- Go ≥ 1.22 with `CGO_ENABLED=0` for reproducible static binaries.
- Node.js ≥ 20 (for Vite/React build) and `npm`.
- Python ≥ 3.10 and `uv` (optional) plus `pip`, `build`, `cibuildwheel`.
- `goreleaser` (or `goreleaser-pro` if we add signing/notarisation).
- Docker CLI (for container images).

## Core Go Binaries
1. **Source layout**
   - `cmd/gateway`: interactive CLI and init generator.
   - `cmd/gatewayd`: long-running HTTP server wrapping the marketplace and token ledger.
2. **Cross-compilation**
   - Target matrix: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`.
   - Build command (example):
     ```bash
     GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -o dist/go/gateway-linux-amd64/gateway ./cmd/gateway
     ```
   - Repeat for `gatewayd` and other targets. Prefer orchestrating via GoReleaser using `builds` + `archives` sections to emit tarballs/zips with checksums.
3. **Config templates**
   - Copy `config/setting.ini` + env overlays into each archive so users have default configs.
   - Document that runtime overrides can be passed via flags (`gateway init --env prod`) or environment variables (e.g. `TOKLIGENCE_EMAIL`).

## Python (`pip` / `uv`) Wrapper
1. **Package skeleton** (under `python-package/`):
   ```
   python-package/
     pyproject.toml
     src/tokgateway/__init__.py
     src/tokgateway/_runner.py
     src/tokgateway/bin/<platform>/gateway
   ```
2. **Entry point**
   - Expose `tokgateway = tokgateway._runner:main` via `console_scripts`.
   - `_runner.py` locates the bundled binary, ensures executable bit, and proxies `argv/stdin/stdout`.
3. **Platform-specific wheels**
   - Use `cibuildwheel` to produce one wheel per target (e.g. `tokgateway-<ver>-cp311-cp311-manylinux_x86_64.whl`).
   - Wheel build job copies the matching Go binary produced by the Go pipeline.
   - Publish a tiny `sdist` that omits binaries so downstream builders can rebuild if desired.
4. **`uv` compatibility**
   - `uv tool install tokgateway` consumes the same wheel metadata, so no extra work beyond uploading to PyPI.
5. **Make target**
   - `make dist-python` runs `cibuildwheel` inside a Python virtualenv, writes artifacts to `dist/python/`.

## Standalone Installers
- **Windows**: use GoReleaser `archives.format=zip` plus optional `nfpm`/`wix` section to produce `.msi`.
- **macOS**: tarball is sufficient initially; notarised `.pkg` can be added later with GoReleaser.
- **Homebrew**: GoReleaser can publish a formula pointing to the macOS tarball.
- **Winget/Scoop**: reuse checksummed archives; automate manifests via release workflow.

## Docker Image
- Build context root; binary copied from `go build` stage into a minimal runtime image (e.g. `gcr.io/distroless/static:nonroot`).
- Multi-stage Dockerfile layout:
  1. Build stage: `golang:1.22` compiles `gateway` and `gatewayd`.
  2. Runtime stage: copy binaries + `config/` + optional static frontends.
- Tag as `ghcr.io/tokligence/gateway:<semver>` in CI.
- `make dist-docker` wraps `docker buildx bake` for multi-arch (linux/amd64, linux/arm64).

## Frontend Bundles (Web & H5)
1. **Shared build**
   - Located in `fe/` (React + Vite). Run `npm ci` once in CI.
2. **Desktop web bundle**
   - `VITE_TARGET=web npm run build` writes to `fe/dist`. Copy to `dist/frontend/web`.
3. **Mobile H5 bundle**
   - Add a companion script in `package.json`:
     ```json
     "build:h5": "VITE_TARGET=h5 npm run build"
     ```
   - Use Vite environment files (`.env.h5`) to adjust viewport, route layout, and asset sizing.
   - Output copied to `dist/frontend/h5`.
4. **Serving options**
   - Upload bundles to CDN/object storage or serve via `gatewayd` with a static-file handler.
   - Provide `.env.production` for API base URLs so H5/web builds point to the correct gateway endpoint.

## Configuration Strategy
- `gateway init` generates `config/setting.ini` plus env-specific overrides (`config/dev`, `config/live`, ...).
- Distributions ship the defaults; users customise via:
  - CLI flags (e.g. `gateway init --env live --base-url https://api.example.com`).
  - Environment variables at runtime (see `cmd/gateway/main.go` for supported keys such as `TOKLIGENCE_EMAIL`, `TOKEN_EXCHANGE_BASE_URL`).
  - Volume-mounting custom configs in Docker (`-v ~/.tokligence:/app/config`).
- Document configuration precedence: CLI flag > env var > config file default.

## Release Workflow
1. Tag (`git tag vX.Y.Z` + push).
2. CI pipeline stages:
   - Run unit/integration tests (`make test`).
   - `make dist-go` (or `goreleaser release --clean --skip-validate` in snapshot mode).
   - `make dist-python` (cibuildwheel matrix across CPython 3.10–3.12).
   - `make dist-frontend` (web + H5 builds; upload as build artifacts).
   - `make dist-docker` (push multi-arch images to GHCR).
3. Publish artifacts:
   - Attach Go archives and frontend bundles to the GitHub release.
   - Upload Python wheels/sdist to TestPyPI → promote to PyPI after smoke tests.
   - Promote Docker image tag from `:rc` to `:latest`.
4. Post-release smoke tests:
   - `pip install --index-url https://test.pypi.org/simple tokgateway` then run `tokgateway init --help`.
   - Download tarball, run `./gateway --help` on Linux/macOS/Windows.
   - Deploy frontend bundle to staging and ensure it hits gatewayd APIs.

## Proposed Make Targets
Add the following (or similar) to `Makefile` to orchestrate builds:

```makefile
.PHONY: dist-go dist-python dist-frontend dist-docker dist-all clean-dist

DIST_VERSION ?= $(shell git describe --tags --dirty --always)
DIST_DIR ?= $(CURDIR)/dist

clean-dist:
	rm -rf $(DIST_DIR)

dist-go:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(DIST_VERSION)" -o $(DIST_DIR)/go/gateway-linux-amd64/gateway ./cmd/gateway
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(DIST_VERSION)" -o $(DIST_DIR)/go/gatewayd-linux-amd64/gatewayd ./cmd/gatewayd
	# Repeat for other GOOS/GOARCH or invoke goreleaser here.

dist-python: dist-go
	python -m pip install --upgrade build cibuildwheel
	python -m cibuildwheel --output-dir $(DIST_DIR)/python/wheels python-package

dist-frontend:
	cd fe && npm ci && VITE_TARGET=web npm run build && cp -r dist $(DIST_DIR)/frontend/web
	cd fe && npm run build:h5 && cp -r dist $(DIST_DIR)/frontend/h5

dist-docker: dist-go dist-frontend
	docker buildx build --platform linux/amd64,linux/arm64 -t ghcr.io/tokligence/gateway:$(DIST_VERSION) --push .

dist-all: clean-dist dist-go dist-python dist-frontend dist-docker
```

## Next Steps
- Implement the additional Make targets and scripts referenced above.
- Add GoReleaser configuration (`.goreleaser.yaml`) to automate archive generation, checksums, and release publishing.
- Extend `fe/package.json` and Vite config to honour `VITE_TARGET=h5` for mobile-specific assets.
- Define distribution-agnostic hook interfaces (e.g. user-provision, API key rotation) so RAG systems like Weaviate or Dgraph can stay in sync regardless of install method.
