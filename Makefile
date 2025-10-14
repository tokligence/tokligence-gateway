SHELL := /bin/bash
PROJECT := tokligence-gateway
TMP_DIR := .tmp
BIN_DIR := bin
PID_FILE := $(TMP_DIR)/$(PROJECT).pid
CLI_LOG := /tmp/$(PROJECT).log

GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache

DIST_DIR ?= $(CURDIR)/dist
DIST_VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || git rev-parse --short HEAD)
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
GO_BINARIES := gateway gatewayd

.PHONY: help build build-gateway build-gatewayd start stop restart run test backend-test frontend-test frontend-lint frontend-ci check fmt lint tidy clean dist-go dist-frontend dist clean-dist ui ui-web ui-h5 ui-dev d-start d-start-detach d-stop d-test d-shell

help:
	@echo "Available targets:" \
		"\n  make build            # Compile gateway and gatewayd binaries" \
		"\n  make start            # Start gateway CLI locally in background" \
		"\n  make stop             # Stop locally running CLI" \
		"\n  make restart          # Restart local CLI" \
		"\n  make run              # Run CLI once in foreground" \
		"\n  make test             # Run all tests (backend + frontend)" \
		"\n  make backend-test     # Run backend Go tests" \
		"\n  make frontend-test    # Run frontend tests" \
		"\n  make frontend-lint    # Run frontend linting" \
		"\n  make frontend-ci      # Run frontend lint + test" \
		"\n  make check            # Smoke test the gateway endpoint" \
		"\n  make dist             # Build distribution packages for all platforms" \
		"\n  make ui               # Start frontend dev server" \
		"\n  make d-start          # Run gateway CLI via Docker (foreground)" \
		"\n  make d-test           # Run Go tests in Docker" \
		"\n  make fmt/lint/tidy    # Common Go tasks" \
		"\n  make clean            # Remove build artifacts"

$(TMP_DIR):
	@mkdir -p $(TMP_DIR)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

# Build targets
build: build-gateway build-gatewayd
	@echo "Built gateway and gatewayd to $(BIN_DIR)/"

build-gateway: $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "-s -w" -o $(BIN_DIR)/gateway ./cmd/gateway

build-gatewayd: $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "-s -w" -o $(BIN_DIR)/gatewayd ./cmd/gatewayd

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
test: backend-test frontend-test

backend-test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

frontend-test:
	cd fe && npm run test

frontend-lint:
	cd fe && npm run lint

frontend-ci: frontend-lint frontend-test

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
