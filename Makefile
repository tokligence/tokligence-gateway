SHELL := /bin/bash
PROJECT := model-free-gateway
TMP_DIR := .tmp
BIN_DIR := bin
PID_FILE := $(TMP_DIR)/$(PROJECT).pid
CLI_LOG := /tmp/$(PROJECT).log

# Go configuration from HEAD
GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache

.PHONY: help d-start d-start-detach d-stop d-test d-shell start stop restart run test fmt lint tidy clean check build backend-test frontend-test frontend-lint frontend-ci

help:
	@echo "Available targets:" \
		"\n  make d-start           # Run gateway CLI via Docker (foreground)" \
		"\n  make d-start-detach    # Run gateway CLI in Docker dev container" \
		"\n  make d-stop            # Stop Docker services" \
		"\n  make d-test            # Run Go tests in Docker" \
		"\n  make d-shell           # Shell into Docker dev container" \
		"\n  make build            # Compile gateway CLI to bin/tokligence" \
		"\n  make check           # Curl the active Token Exchange endpoint" \
		"\n  make start            # Start CLI locally in background" \
		"\n  make stop             # Stop locally running CLI" \
		"\n  make restart          # Restart local CLI" \
		"\n  make run              # Run CLI once in foreground" \
		"\n  make test             # Run go test ./... on host" \
		"\n  make backend-test     # Run backend tests with custom Go settings" \
		"\n  make frontend-test    # Run frontend tests" \
		"\n  make frontend-lint    # Run frontend linting" \
		"\n  make frontend-ci      # Run frontend lint + test" \
		"\n  make fmt/lint/tidy/test     # Common Go tasks" \
		"\n  make clean                  # Remove build artifacts"

$(TMP_DIR):
	@mkdir -p $(TMP_DIR)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

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

# Local targets

start: $(TMP_DIR)
	@if [ -f $(PID_FILE) ] && kill -0 $$(cat $(PID_FILE)) 2>/dev/null; then \
		echo "$(PROJECT) CLI already running with PID $$(cat $(PID_FILE))"; \
	else \
		env TOKEN_EXCHANGE_BASE_URL=$${TOKEN_EXCHANGE_BASE_URL:-http://localhost:8080} nohup go run ./cmd/gateway >$(CLI_LOG) 2>&1 & echo $$! > $(PID_FILE); \
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
	@TOKEN_EXCHANGE_BASE_URL=$${TOKEN_EXCHANGE_BASE_URL:-http://localhost:8080} go run ./cmd/gateway

build: $(BIN_DIR)
	@go build -o $(BIN_DIR)/tokligence ./cmd/gateway
	@echo "Built $(BIN_DIR)/tokligence"

test:
	@go test ./...

check:
	@./scripts/smoke.sh

# Backend/frontend test targets from HEAD
backend-test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

frontend-test:
	cd fe && npm run test

frontend-lint:
	cd fe && npm run lint

frontend-ci: frontend-lint frontend-test

# Common utilities

fmt:
	@go fmt ./...

lint:
	@go vet ./...

tidy:
	@go mod tidy

clean:
	@rm -rf $(TMP_DIR) .gocache .gomodcache $(CLI_LOG) $(BIN_DIR)
