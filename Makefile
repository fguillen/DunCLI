GO        ?= go
PKG       := ./...
BIN_DIR   := bin
BINARY    := $(BIN_DIR)/dun
OGEN_SPEC := docs/backend/openapi.yaml
OGEN_OUT  := internal/api/gen
OGEN_PKG  := gen

.PHONY: all generate build test lint run tidy clean help

all: build

## generate: regenerate the typed API client from the backend OpenAPI spec.
generate:
	$(GO) run github.com/ogen-go/ogen/cmd/ogen \
		-package $(OGEN_PKG) \
		-target $(OGEN_OUT) \
		-clean \
		-config ogen.yml \
		$(OGEN_SPEC)

## build: compile the dun binary into $(BIN_DIR).
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BINARY) ./cmd/dun

## test: run the full test suite.
test:
	$(GO) test $(PKG)

## lint: run golangci-lint.
lint:
	golangci-lint run

## run: launch the TUI from source.
run:
	$(GO) run ./cmd/dun tui

## tidy: refresh go.mod / go.sum.
tidy:
	$(GO) mod tidy

## clean: remove build artifacts.
clean:
	rm -rf $(BIN_DIR)

## help: list available targets.
help:
	@grep -E '^##' Makefile | sed -e 's/## /  /'
