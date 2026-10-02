GO_HOST ?= $(shell go env GOOS 2>/dev/null)-$(shell go env GOARCH 2>/dev/null)
GO_CACHE ?= /tmp/internkim-go-cache-$(GO_HOST)
GO_MOD_CACHE ?= /tmp/internkim-go-mod-cache-$(GO_HOST)
RELAY_TARGET ?=

.PHONY: build build-maild build-relay build-company-host verify-generated-protocol check test doctor deps-sim deps-browser prepare-blueclaw-runtime-builder prepare-blueclaw-runtime-base prepare-blueclaw-payload prepare-buzz-relay prepare-buzz-relay-linux smoke-blueclaw-runtime-lab smoke-blueclaw-runtime-lab-fast setup-sim fleet-gate sim-gate verify-browser

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

doctor: build
	./internkim doctor

deps-sim:
	@echo "install the container CLI from https://github.com/apple/container/releases"

deps-browser:
	cd web && bun install
	cd web && bunx playwright install chromium

prepare-blueclaw-runtime-builder: build
	./internkim lab runtime-builder-prepare

prepare-blueclaw-runtime-base:
	if [ "$$(uname -s)" = "Linux" ]; then GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-runtime --builder local; else GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-runtime --builder container; fi

prepare-blueclaw-payload: verify-generated-protocol
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-payload

prepare-buzz-relay:
	tools/prepare-buzz-relay

prepare-buzz-relay-linux:
	tools/prepare-buzz-relay --target linux

smoke-blueclaw-runtime-lab: build
	./internkim setup --sim --only blueclaw-runtime-base,blueclaw-payload,skills,services,users-sync --force-all --verify

smoke-blueclaw-runtime-lab-fast: build
	./internkim setup --sim --only binaries,blueclaw-runtime-base,blueclaw-payload,services --verify

build-litert-lm-main:
	tools/build-litert-lm-main

setup-sim: build
	./internkim setup --sim

fleet-gate: build
	./internkim dev fleet run

sim-gate: fleet-gate

verify-browser: build
	./internkim verify browser --local
	./internkim verify browser --public
