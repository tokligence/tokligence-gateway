SHELL := /bin/bash
PROJECT := model-free-gateway
TMP_DIR := .tmp
PID_FILE := $(TMP_DIR)/$(PROJECT).pid
CLI_LOG := /tmp/$(PROJECT).log

.PHONY: help d-start d-start-detach d-stop d-test d-shell start stop restart run test fmt lint tidy clean check

help:
	@echo "Available targets:" \
		"\n  make d-start           # Run gateway CLI via Docker (foreground)" \
		"\n  make d-start-detach    # Run gateway CLI in Docker dev container" \
		"\n  make d-stop            # Stop Docker services" \
		"\n  make d-test            # Run Go tests in Docker" \
		"\n  make d-shell           # Shell into Docker dev container" \
		"\n  make check           # Curl the active Token Exchange endpoint" \
		"\n  make start            # Start CLI locally in background" \
		"\n  make stop             # Stop locally running CLI" \
		"\n  make restart          # Restart local CLI" \
		"\n  make run              # Run CLI once in foreground" \
		"\n  make test             # Run go test ./... on host" \
		"\n  make fmt/lint/tidy/test     # Common Go tasks" \
		"\n  make clean                  # Remove build artifacts"

$(TMP_DIR):
	@mkdir -p $(TMP_DIR)

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
		env TOKEN_EXCHANGE_BASE_URL=$${TOKEN_EXCHANGE_BASE_URL:-http://localhost:8080} nohup go run ./cmd/mfg >$(CLI_LOG) 2>&1 & echo $$! > $(PID_FILE); \
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
	@TOKEN_EXCHANGE_BASE_URL=$${TOKEN_EXCHANGE_BASE_URL:-http://localhost:8080} go run ./cmd/mfg

test:
	@go test ./...

check:
	@./scripts/smoke.sh

# Common utilities

fmt:
	@go fmt ./...

lint:
	@go vet ./...

tidy:
	@go mod tidy


clean:
	@rm -rf $(TMP_DIR) .gocache .gomodcache $(CLI_LOG)
