SHELL := /bin/bash
PROJECT := tokligence-gateway
TMP_DIR := .tmp
BIN_DIR := bin
PID_FILE := $(TMP_DIR)/$(PROJECT).pid
CLI_LOG := /tmp/$(PROJECT).log
# gatewayd daemon helpers
DAEMON_PID_FILE := $(TMP_DIR)/gatewayd.pid
DAEMON_OUT := /tmp/$(PROJECT)-daemon.out

GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache

DIST_DIR ?= $(CURDIR)/dist
DIST_VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || git rev-parse --short HEAD)
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
GO_BINARIES := gateway gatewayd

GIT_COMMIT := $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
# Build timezone: can be overridden with TZ=... make build
BUILD_TZ ?= Asia/Singapore
BUILD_TIME := $(shell TZ=$(BUILD_TZ) date +"%Y-%m-%dT%H:%M:%S%z")
LD_FLAGS = -s -w -X main.buildVersion=$(DIST_VERSION) -X main.buildCommit=$(GIT_COMMIT) -X main.buildBuiltAt=$(BUILD_TIME)

.PHONY: help build build-gateway build-gatewayd start stop restart run test be-test bridge-test fe-test frontend-lint frontend-ci check fmt lint tidy clean dist-go dist-frontend dist clean-dist ui ui-web ui-h5 ui-dev d-start d-start-detach d-stop d-test d-shell \
	gd-start gd-stop gd-restart gd-status \
	gd-force-restart \
	anthropic-sidecar ansi openai-delegate ode \
	gfr gds gdx gdr gst bg bgd bt ft fl fci ds dx dt dsh dg dfr 

help:
	@echo "Available targets:" \
		"\n  make build            # Compile gateway and gatewayd binaries" \
		"\n  make start            # Start gateway CLI locally in background" \
		"\n  make stop             # Stop locally running CLI" \
		"\n  make restart          # Restart local CLI" \
		"\n  make run              # Run CLI once in foreground" \
		"\n  make test             # Run all tests (bridge + backend + frontend)" \
		"\n  make bridge-test      # Run API translation (bridge) tests" \
		"\n  make be-test (bt)     # Run backend Go tests" \
		"\n  make fe-test (ft)     # Run frontend tests" \
		"\n  make frontend-lint (fl)    # Run frontend linting" \
		"\n  make frontend-ci (fci)     # Run frontend lint + test" \
		"\n  make check            # Smoke test the gateway endpoint" \
		"\n  make dist             # Build distribution packages for all platforms" \
		"\n  make ui               # Start frontend dev server" \
		"\n  make d-start (ds)     # Run gateway CLI via Docker (foreground)" \
		"\n  make d-test (dt)      # Run Go tests in Docker" \
		"\n  make fmt/lint/tidy    # Common Go tasks" \
		"\n  make clean            # Remove build artifacts" \
		"\n" \
		"\nGatewayd daemon shortcuts:" \
		"\n  make gd-start (gds)                          # Start gatewayd daemon" \
		"\n  make gd-stop (gdx)                           # Stop gatewayd daemon" \
		"\n  make gd-restart (gdr)                        # Restart gatewayd daemon" \
		"\n  make gd-force-restart (gfr)                  # Force kill any process on :8081, rotate logs, restart gatewayd" \
		"\n  make gd-status (gst)                         # Show gatewayd status and port" \
		"\n  make anthropic-sidecar (ansi)                # Start gatewayd for Codex→Anthropic (/v1/responses mapping, no OpenAI delegation)" \
		"\n  make openai-delegate (ode)                   # Start gatewayd with OpenAI /v1/responses delegation (gpt*/o1*)" \
		"\n" \
		"\nBuild shortcuts:" \
		"\n  make build-gateway (bg)                      # Build gateway CLI only" \
		"\n  make build-gatewayd (bgd)                    # Build gatewayd daemon only" \
		"\n" \
		"\nDocker shortcuts:" \
		"\n  make d-stop (dx)                             # Stop Docker containers" \
		"\n  make d-shell (dsh)                           # Open shell in Docker container" \
		"\n" \
		"\nDistribution shortcuts:" \
		"\n  make dist-go (dg)                            # Build Go binaries for all platforms" \
		"\n  make dist-frontend (dfr)                     # Build frontend for distribution"

$(TMP_DIR):
	@mkdir -p $(TMP_DIR)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Build targets
build: build-gateway build-gatewayd
	@echo "Built gateway and gatewayd to $(BIN_DIR)/"

build-gateway: $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "$(LD_FLAGS)" -o $(BIN_DIR)/gateway ./cmd/gateway

build-gatewayd: $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "$(LD_FLAGS)" -o $(BIN_DIR)/gatewayd ./cmd/gatewayd

# Docker targets
d-start:
	@docker compose run --rm cli

d-start-detach:
	@docker compose up -d dev

d-stop:
	@docker compose down

d-test:
	@docker compose run --rm test

d-shell:
	@docker compose run --rm dev bash

# Local runtime targets
start: $(TMP_DIR)
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "$(PROJECT) CLI already running with PID $$(cat $(PID_FILE))"; \
	else \
		env TOKLIGENCE_BASE_URL=$${TOKLIGENCE_BASE_URL:-http://localhost:8080} nohup go run ./cmd/gateway >$(CLI_LOG) 2>&1 & echo $$! > $(PID_FILE); \
		echo "Started $(PROJECT) CLI (PID $$(cat $(PID_FILE)))"; \
	fi

stop:
	@if [ -f $(PID_FILE) ]; then \
		PID=$$(cat $(PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then \
			kill $$PID && echo "Stopped $(PROJECT) CLI (PID $$PID)"; \
		else \
			echo "Process $$PID not running"; \
		fi; \
		rm -f $(PID_FILE); \
	else \
		echo "No PID file at $(PID_FILE)"; \
	fi

restart: stop start

run:
	@TOKLIGENCE_BASE_URL=$${TOKLIGENCE_BASE_URL:-http://localhost:8080} go run ./cmd/gateway

check:
	@./scripts/smoke.sh

# Test targets
test: be-test fe-test

be-test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

fe-test:
	cd fe && npm run test

frontend-lint:
	cd fe && npm run lint

frontend-ci: frontend-lint fe-test

# Frontend dev targets
ui:
	cd fe && npm run dev

ui-web:
	cd fe && npm run build:web && npx --yes serve dist

ui-h5:
	cd fe && npm run build:h5 && npx --yes serve dist

ui-dev:
	cd fe && npm run dev

# Distribution targets
dist-go:
	@mkdir -p "$(DIST_DIR)"
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		for bin in $(GO_BINARIES); do \
			output="$(DIST_DIR)/$${bin}-$(DIST_VERSION)-$${os}-$${arch}"; \
			if [ "$$os" = "windows" ]; then output="$${output}.exe"; fi; \
			echo "Building $$output..."; \
			GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 $(GO) build \
				-ldflags "-s -w -X main.Version=$(DIST_VERSION)" \
				-o "$$output" ./cmd/$$bin || exit 1; \
		done; \
	done
	@echo "Go binaries built in $(DIST_DIR)/"

clean-dist:
	rm -rf "$(DIST_DIR)"

dist-frontend:
	cd fe && npm run build:web
	mkdir -p "$(DIST_DIR)/frontend/web"
	cp -R fe/dist/. "$(DIST_DIR)/frontend/web/"
	rm -rf fe/dist
	cd fe && npm run build:h5
	mkdir -p "$(DIST_DIR)/frontend/h5"
	cp -R fe/dist/. "$(DIST_DIR)/frontend/h5/"
	rm -rf fe/dist

dist:
	$(MAKE) clean-dist
	$(MAKE) dist-go
	$(MAKE) dist-frontend

# Utility targets
fmt:
	@go fmt ./...

lint:
	@go vet ./...

tidy:
	@go mod tidy

clean:
	@rm -rf $(BIN_DIR) $(TMP_DIR) $(DIST_DIR) .gocache .gomodcache $(CLI_LOG)

adp:
	rm logs/*
	make build
	./bin/gatewayd

# -----------------
# gatewayd helpers
# -----------------

ensure-tmp:
	@mkdir -p $(TMP_DIR) logs

gd-start: build ensure-tmp
	@if [ -f $(DAEMON_PID_FILE) ] && kill -0 $$(cat $(DAEMON_PID_FILE)) 2>/dev/null; then \
		echo "gatewayd already running with PID $$(cat $(DAEMON_PID_FILE))"; \
		exit 0; \
	fi
	@echo "Starting gatewayd with default env (override by exporting TOKLIGENCE_* vars)"
	@nohup /bin/bash -lc 'set -a; [ -f .env ] && source .env; set +a; \
		env TOKLIGENCE_LOG_LEVEL=$${TOKLIGENCE_LOG_LEVEL:-debug} \
		TOKLIGENCE_MARKETPLACE_ENABLED=$${TOKLIGENCE_MARKETPLACE_ENABLED:-false} \
		TOKLIGENCE_ROUTES="$${TOKLIGENCE_ROUTES:-claude*=>anthropic,gpt*=>anthropic}" \
		TOKLIGENCE_RESPONSES_DELEGATE_OPENAI=$${TOKLIGENCE_RESPONSES_DELEGATE_OPENAI:-never} \
		./bin/gatewayd' > $(DAEMON_OUT) 2>&1 & echo $$! > $(DAEMON_PID_FILE)
	@echo "gatewayd started (PID $$(cat $(DAEMON_PID_FILE)))"

gd-stop:
	@if [ -f $(DAEMON_PID_FILE) ]; then \
		PID=$$(cat $(DAEMON_PID_FILE)); \
		if kill -0 $$PID 2>/dev/null; then kill $$PID && echo "Stopped gatewayd (PID $$PID)"; else echo "Process $$PID not running"; fi; \
		rm -f $(DAEMON_PID_FILE); \
	else \
		pids=$$(pgrep -f "/bin/gatewayd" || true); \
		if [ -n "$$pids" ]; then echo "Killing $$pids"; kill $$pids; else echo "gatewayd not running"; fi; \
	fi

gd-restart: gd-stop gd-start

gd-force-restart:
	@pids=$$(lsof -t -iTCP:8081 -sTCP:LISTEN 2>/dev/null || true); \
	if [ -n "$$pids" ]; then \
		echo "Force killing processes on :8081 -> $$pids"; \
		kill -9 $$pids || true; \
	fi
	@rm -f $(DAEMON_PID_FILE)
	@rm -f logs/dev-gatewayd.log logs/dev-gatewayd-*.log
	@$(MAKE) gd-start

gd-status:
	@echo "Listening ports:" && ss -ltnp | grep gatewayd || true; \
	echo "Daemon out: $(DAEMON_OUT)" && tail -n 50 $(DAEMON_OUT) || true

# Convenience profiles

# Codex → Anthropic via /v1/responses mapping; disables OpenAI delegation; sidecar bridge on
anthropic-sidecar ansi: build ensure-tmp
	@if [ -z "$$TOKLIGENCE_ANTHROPIC_API_KEY" ]; then echo "[WARN] TOKLIGENCE_ANTHROPIC_API_KEY not set"; fi
	@if [ -f $(DAEMON_PID_FILE) ] && kill -0 $$(cat $(DAEMON_PID_FILE)) 2>/dev/null; then \
		echo "gatewayd already running with PID $$(cat $(DAEMON_PID_FILE))"; exit 0; \
	fi
	@echo "Starting gatewayd (anthropic sidecar; no OpenAI delegation)"
	@nohup /bin/bash -lc 'set -a; [ -f .env ] && source .env; set +a; \
		env TOKLIGENCE_LOG_LEVEL=debug TOKLIGENCE_MARKETPLACE_ENABLED=false \
		TOKLIGENCE_RESPONSES_DELEGATE_OPENAI=never \
		TOKLIGENCE_ANTHROPIC_MESSAGES_MODE=sidecar \
		TOKLIGENCE_ROUTES="claude*=>anthropic,gpt-*=>openai,loopback=>loopback" \
		./bin/gatewayd' > $(DAEMON_OUT) 2>&1 & echo $$! > $(DAEMON_PID_FILE)
	@echo "gatewayd started (PID $$(cat $(DAEMON_PID_FILE)))"

# Codex → OpenAI via /v1/responses (delegate)
openai-delegate ode: build ensure-tmp
	@if [ -z "$$TOKLIGENCE_OPENAI_API_KEY" ]; then echo "[WARN] TOKLIGENCE_OPENAI_API_KEY not set"; fi
	@if [ -f $(DAEMON_PID_FILE) ] && kill -0 $$(cat $(DAEMON_PID_FILE)) 2>/dev/null; then \
		echo "gatewayd already running with PID $$(cat $(DAEMON_PID_FILE))"; exit 0; \
	fi
	@echo "Starting gatewayd (OpenAI /v1/responses delegation)"
	@nohup /bin/bash -lc 'set -a; [ -f .env ] && source .env; set +a; \
		env TOKLIGENCE_LOG_LEVEL=debug TOKLIGENCE_MARKETPLACE_ENABLED=false \
		TOKLIGENCE_RESPONSES_DELEGATE_OPENAI=always \
		./bin/gatewayd' > $(DAEMON_OUT) 2>&1 & echo $$! > $(DAEMON_PID_FILE)
	@echo "gatewayd started (PID $$(cat $(DAEMON_PID_FILE)))"

# ---------------------
# Shorthand aliases
# ---------------------
# gatewayd shortcuts
gfr: gd-force-restart
gds: gd-start
gdx: gd-stop
gdr: gd-restart
gst: gd-status

# build shortcuts
bg: build-gateway
bgd: build-gatewayd

# test shortcuts
bt: be-test
ft: fe-test
fl: frontend-lint
fci: frontend-ci

# docker shortcuts
ds: d-start
dx: d-stop
dt: d-test
dsh: d-shell

# distribution shortcuts
dg: dist-go
dfr: dist-frontend
