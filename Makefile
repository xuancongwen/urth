BIN     := bin/urth
PKG     := ./cmd/urth
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X urth/internal/version.Version=$(VERSION) -X urth/internal/version.Commit=$(COMMIT)

.PHONY: help build run test vet fmt fmtcheck check clean

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ## Build a static binary into bin/
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

run: ## Run the server from source with config.yaml
	go run $(PKG) -config config.yaml

test: ## Run unit tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -w .

fmtcheck: ## Fail if any Go file is not gofmt-formatted
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

check: fmtcheck vet test ## Format check, vet, and test

clean: ## Remove build output
	rm -rf bin
