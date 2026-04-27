GO_CACHE ?= /tmp/internkim-go-cache
BLUECLAW_GO_CACHE ?= /tmp/blueclaw-go-cache

.PHONY: build build-companion check test doctor deps-sim deps-browser deps-graphiti setup-sim verify-api verify-browser verify-graphiti-local

build:
	GOCACHE=$(GO_CACHE) go build -o internkim ./cmd/internkim

build-companion:
	GOCACHE=$(GO_CACHE) go build -o internkim-companion ./cmd/internkim-companion

check: build build-companion
	cd web && bun run check
	GOCACHE=$(GO_CACHE) go test ./...
	cd .dependency/blueclaw && GOCACHE=$(BLUECLAW_GO_CACHE) go test ./...

test: check

doctor: build
	./internkim doctor

deps-sim:
	brew install cirruslabs/cli/tart

deps-browser:
	cd web && bun install
	cd web && bunx playwright install chromium

deps-graphiti:
	cd .dependency/blueclaw && test -x .venv-graphiti/bin/python || uv venv .venv-graphiti
	cd .dependency/blueclaw && uv pip install --python .venv-graphiti/bin/python -r tools/graphiti_memoryd/requirements.txt

setup-sim: build
	./internkim setup --sim

verify-api: build
	./internkim verify api
	./internkim verify mattermost

verify-browser: build
	./internkim verify browser --local
	./internkim verify browser --public

verify-graphiti-local:
	tools/verify-graphiti-local
