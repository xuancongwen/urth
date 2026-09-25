BIN     := bin/urth
PKG     := ./cmd/urth
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
LDFLAGS := -s -w -X urth/internal/version.Version=$(VERSION) -X urth/internal/version.Commit=$(COMMIT)

.PHONY: help build run test vet fmt fmtcheck content check clean deploy

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

content: ## Load, lint, and report on data/ without starting a server
	go run $(PKG) check -config config.yaml

check: fmtcheck vet test content ## Format check, vet, test, and content check

clean: ## Remove build output
	rm -rf bin

deploy: ## Build and push to the LXC named in deploy/deploy.env
	deploy/deploy.sh $(DEPLOY_ARGS)
