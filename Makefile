GO_HOST ?= $(shell go env GOOS 2>/dev/null)-$(shell go env GOARCH 2>/dev/null)
GO_CACHE ?= /tmp/internkim-go-cache-$(GO_HOST)
GO_MOD_CACHE ?= /tmp/internkim-go-mod-cache-$(GO_HOST)
RELAY_TARGET ?=

.PHONY: build build-maild build-relay build-company-host generate-protocol verify-generated-protocol check test deps-browser prepare-buzz-relay prepare-buzz-relay-linux

build: verify-generated-protocol
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o internkim ./cmd/internkim

build-maild:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o internkim-maild ./cmd/internkim-maild

build-relay:
	cd host/relay && bun install --frozen-lockfile
	cd host/relay && bun build --compile $(if $(RELAY_TARGET),--target=$(RELAY_TARGET)) --outfile ../../internkim-relay relay.ts

build-company-host:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o internkim-host ./cmd/internkim-host

generate-protocol:
	cd .dependency/blueclaw/protocol && bun install --frozen-lockfile
	cd .dependency/blueclaw/protocol && bun run generate
	cd web && bun install --frozen-lockfile
	cd web && bun run generate:protocol

verify-generated-protocol:
	cd .dependency/blueclaw/protocol && bun install --frozen-lockfile
	cd .dependency/blueclaw/protocol && bun run generate:check
	cd web && bun install --frozen-lockfile
	cd web && bun run generate:protocol --check

check:
	tools/verify --all

test: check

deps-browser:
	cd web && bun install
	cd web && bunx playwright install chromium

prepare-buzz-relay:
	tools/prepare-buzz-relay

prepare-buzz-relay-linux:
	tools/prepare-buzz-relay --target linux
