SHELL := /bin/sh

.DEFAULT_GOAL := help

CACHE_DIR := $(CURDIR)/.cache
BIN_DIR := $(CACHE_DIR)/bin
GOCACHE := $(CACHE_DIR)/go-build
GOMODCACHE := $(CACHE_DIR)/go-mod
GOLANGCI_LINT_CACHE := $(CACHE_DIR)/golangci-lint
GOLANGCI_LINT_VERSION := v2.11.4

export PATH := $(BIN_DIR):$(PATH)

.PHONY: help
help:
	@printf '%s\n' 'Canterbury development targets:'
	@printf '%s\n' '  make setup           Install Node dependencies, Go modules, and Go tools'
	@printf '%s\n' '  make deps            Install project dependencies'
	@printf '%s\n' '  make tools           Install project tools'
	@printf '%s\n' '  make check           Run the full repository check'
	@printf '%s\n' '  make format          Format repository files'
	@printf '%s\n' '  make test            Run Go tests'
	@printf '%s\n' '  make lint            Run Go linting'
	@printf '%s\n' '  make smoke-auth      Run local auth smoke tests'
	@printf '%s\n' '  make proto-generate  Regenerate protobuf outputs'

.PHONY: setup
setup: deps tools

.PHONY: deps
deps: deps-node deps-go

.PHONY: deps-node
deps-node:
	npm ci
	npm --prefix sync ci

.PHONY: deps-go
deps-go:
	GOMODCACHE=$(GOMODCACHE) go mod download

.PHONY: tools
tools: tools-go
	@printf '%s\n' 'Project tools are installed.'

.PHONY: tools-go
tools-go:
	@mkdir -p $(BIN_DIR) $(CACHE_DIR)/tools
	@if ! $(BIN_DIR)/golangci-lint version 2>/dev/null | grep -q '$(GOLANGCI_LINT_VERSION:v%=%)'; then \
		printf '%s\n' 'Installing golangci-lint $(GOLANGCI_LINT_VERSION)...'; \
		GOBIN=$(BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi

.PHONY: check
check: tools-go
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) npm run check

.PHONY: format
format:
	npm run format

.PHONY: test
test:
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) npm run test:go

.PHONY: lint
lint: tools-go
	GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOLANGCI_LINT_CACHE=$(GOLANGCI_LINT_CACHE) npm run lint:go

.PHONY: smoke-auth
smoke-auth:
	npm run smoke:auth

.PHONY: proto-generate
proto-generate:
	npm run proto:generate
