# pod-log-preserver Makefile
#
# Common developer entry points. The Go toolchain, golangci-lint, and helm are
# pinned in aqua.yaml and invoked by bare name; CI runs these same targets, so
# local and CI use byte-identical tools (see .github/workflows/ci.yaml).

BINARY   := pod-log-preserver
CMD      := ./cmd/pod-log-preserver
BIN_DIR  := bin
GO       ?= go

.DEFAULT_GOAL := help

# aqua (aqua.yaml) is the single source of truth for every CLI's version. Prepend
# aqua's bin dir to PATH so `make` finds the pinned tools even when the interactive
# shell PATH does not already include it. No-op when aqua is absent.
AQUA_ROOT := $(shell command -v aqua >/dev/null 2>&1 && aqua root-dir)
ifneq ($(AQUA_ROOT),)
export PATH := $(AQUA_ROOT)/bin:$(PATH)
endif

## aqua-tools: link the pinned CLIs so bare-name invocations resolve
# `aqua install --only-link` creates the command symlinks for every tool in
# aqua.yaml (cheap, offline, idempotent), so tool-using targets resolve the bare
# names without a manual `aqua i`. The binaries are still fetched lazily at their
# pinned versions on first use.
.PHONY: aqua-tools
aqua-tools:
	@command -v aqua >/dev/null 2>&1 || { \
		echo "aqua not found — install it (https://aquaproj.github.io) so the pinned CLIs in aqua.yaml resolve"; \
		exit 1; \
	}
	@aqua install --only-link

## build: compile the binary into bin/
.PHONY: build
build: aqua-tools
	$(GO) build -o $(BIN_DIR)/$(BINARY) $(CMD)

## test: run the test suite with the race detector
.PHONY: test
test: aqua-tools
	$(GO) test -race ./...

## vet: run go vet across all packages
.PHONY: vet
vet: aqua-tools
	$(GO) vet ./...

## lint: run golangci-lint (pinned in aqua.yaml)
.PHONY: lint
lint: aqua-tools
	golangci-lint run ./...

## fmt: format all Go sources in place
.PHONY: fmt
fmt: aqua-tools
	$(GO) fmt ./...

## tidy: sync go.mod/go.sum with the source
.PHONY: tidy
tidy: aqua-tools
	$(GO) mod tidy

## clean: remove build output
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

## e2e-container: build the image and run the container e2e harness (needs Docker)
.PHONY: e2e-container
e2e-container: aqua-tools
	docker build -t pod-log-preserver:e2e .
	$(GO) test -tags e2e -count=1 -v -timeout 20m ./test/e2e/container/...

## help: list available targets
.PHONY: help
help:
	@grep -E '^## [a-z-]+:' $(MAKEFILE_LIST) | sed 's/^## /  /' | sort
