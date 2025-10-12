GO ?= /usr/local/go/bin/go
GOCACHE ?= $(CURDIR)/.gocache
GOMODCACHE ?= $(CURDIR)/.gomodcache

.PHONY: backend-test frontend-test frontend-lint frontend-ci test

backend-test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) $(GO) test ./...

frontend-test:
	cd fe && npm run test

frontend-lint:
	cd fe && npm run lint

frontend-ci: frontend-lint frontend-test

# Convenience aggregate target
test: backend-test frontend-test
