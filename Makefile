# pod-log-preserver Makefile
#
# Common developer entry points. CI runs the same underlying `go` and
# `golangci-lint` commands (see .github/workflows/ci.yaml); these targets are
# for local use and documentation.

BINARY   := pod-log-preserver
CMD      := ./cmd/pod-log-preserver
BIN_DIR  := bin
GO       ?= go

.DEFAULT_GOAL := help

## build: compile the binary into bin/
.PHONY: build
build:
	$(GO) build -o $(BIN_DIR)/$(BINARY) $(CMD)

## test: run the test suite with the race detector
.PHONY: test
test:
	$(GO) test -race ./...

## vet: run go vet across all packages
.PHONY: vet
vet:
	$(GO) vet ./...

## lint: run golangci-lint (must be installed; see CI for the pinned version)
.PHONY: lint
lint:
	golangci-lint run ./...

## fmt: format all Go sources in place
.PHONY: fmt
fmt:
	$(GO) fmt ./...

## tidy: sync go.mod/go.sum with the source
.PHONY: tidy
tidy:
	$(GO) mod tidy

## clean: remove build output
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

## help: list available targets
.PHONY: help
help:
	@grep -E '^## [a-z]+:' $(MAKEFILE_LIST) | sed 's/^## /  /' | sort
