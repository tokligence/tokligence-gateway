SHELL := /bin/bash

GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache

DIST_DIR ?= $(CURDIR)/dist
DIST_VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || git rev-parse --short HEAD)
PLATFORMS ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64
GO_BINARIES := gateway gatewayd
BIN_DIR ?= $(CURDIR)/bin

.PHONY: build build-gateway build-gatewayd backend-test frontend-test frontend-lint frontend-ci test dist-go dist-frontend dist clean-dist ui ui-h5 ui-dev

# Simple build commands for local development
build: build-gateway build-gatewayd
	@echo "Built gateway and gatewayd to $(BIN_DIR)/"

build-gateway:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "-s -w" -o $(BIN_DIR)/gateway ./cmd/gateway

build-gatewayd:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GO) build -ldflags "-s -w" -o $(BIN_DIR)/gatewayd ./cmd/gatewayd

clean:
	rm -rf $(BIN_DIR)

backend-test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

frontend-test:
	cd fe && npm run test

frontend-lint:
	cd fe && npm run lint

frontend-ci: frontend-lint frontend-test

# Convenience aggregate target
test: backend-test frontend-test

ui:
	cd fe && npm run dev

ui-web:
	cd fe && npm run build:web && npx --yes serve dist

ui-h5:
	cd fe && npm run build:h5 && npx --yes serve dist

clean-dist:
	rm -rf $(DIST_DIR)

dist-go:
	@set -euo pipefail; \
	for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*}; \
		GOARCH=$${platform#*/}; \
		OUTDIR="$(DIST_DIR)/go/$${GOOS}-$${GOARCH}"; \
		mkdir -p "$${OUTDIR}"; \
		for bin in $(GO_BINARIES); do \
			OUTPUT="$${OUTDIR}/$${bin}"; \
			if [[ $$GOOS == windows ]]; then OUTPUT="$${OUTPUT}.exe"; fi; \
			CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH $(GO) build -ldflags "-s -w" -o "$${OUTPUT}" ./cmd/$${bin}; \
		done; \
		rm -rf "$${OUTDIR}/config"; \
		cp -R config "$${OUTDIR}/config"; \
	done

dist-frontend:
	@set -euo pipefail; \
	cd fe; \
	npm ci; \
	npm run build:web; \
	rm -rf "$(DIST_DIR)/frontend/web"; \
	mkdir -p "$(DIST_DIR)/frontend/web"; \
	cp -R dist/. "$(DIST_DIR)/frontend/web/"; \
	npm run build:h5; \
	rm -rf "$(DIST_DIR)/frontend/h5"; \
	mkdir -p "$(DIST_DIR)/frontend/h5"; \
	cp -R dist/. "$(DIST_DIR)/frontend/h5/"; \
	rm -rf dist

dist:
	$(MAKE) clean-dist
	$(MAKE) dist-go
	$(MAKE) dist-frontend
