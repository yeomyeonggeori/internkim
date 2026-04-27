GO_CACHE ?= /tmp/internkim-go-cache
GO_MOD_CACHE ?= /tmp/internkim-go-mod-cache
BLUECLAW_GO_CACHE ?= /tmp/blueclaw-go-cache
COMPANION_TARGET_TRIPLE ?= $(shell rustc -vV 2>/dev/null | sed -n 's/host: //p')
AGENT_BROWSER_VERSION ?= 0.26.0

.PHONY: build build-companion build-companion-shell check test doctor deps-sim deps-browser deps-companion deps-companion-browser deps-graphiti setup-sim verify-api verify-browser verify-graphiti-local

build:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o internkim ./cmd/internkim

build-companion:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o internkim-companion ./cmd/internkim-companion
	mkdir -p companion/src-tauri/binaries
	cp internkim-companion companion/src-tauri/binaries/internkim-companion
	if [ -n "$(COMPANION_TARGET_TRIPLE)" ]; then cp internkim-companion companion/src-tauri/binaries/internkim-companion-$(COMPANION_TARGET_TRIPLE); fi
	tools/prepare-companion-agent-browser companion/src-tauri/binaries "$(COMPANION_TARGET_TRIPLE)" "$(AGENT_BROWSER_VERSION)"

build-companion-shell: build-companion
	cd companion && bun run build:tauri

check: build build-companion
	cd companion && bun run check
	cd web && bun run check
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...
	cd .dependency/blueclaw && GOCACHE=$(BLUECLAW_GO_CACHE) go test ./...

test: check

doctor: build
	./internkim doctor

deps-sim:
	brew install cirruslabs/cli/tart

deps-browser:
	cd web && bun install
	cd web && bunx playwright install chromium

deps-companion:
	cd companion && bun install

deps-companion-browser:
	tools/prepare-companion-agent-browser companion/src-tauri/binaries "$(COMPANION_TARGET_TRIPLE)" "$(AGENT_BROWSER_VERSION)"
	companion/src-tauri/binaries/agent-browser --version
	companion/src-tauri/binaries/agent-browser install

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
