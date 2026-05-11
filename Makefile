GO_CACHE ?= /tmp/internkim-go-cache
GO_MOD_CACHE ?= /tmp/internkim-go-mod-cache-v2
BLUECLAW_GO_CACHE ?= /tmp/blueclaw-go-cache
COMPANION_TARGET_TRIPLE ?= $(shell rustc -vV 2>/dev/null | sed -n 's/host: //p')
AGENT_BROWSER_VERSION ?= 0.26.0
COMPANION_BETA_DIST ?= dist/companion
COMPANION_BETA_MACOS_ARTIFACT ?= internkim-companion-beta-macos-aarch64.dmg

.PHONY: build build-companion build-companion-shell package-companion-beta check test doctor deps-sim deps-browser deps-companion deps-companion-browser prepare-blueclaw-runtime-builder prepare-blueclaw-runtime-base prepare-blueclaw-payload smoke-blueclaw-runtime-tart deps-graphiti setup-sim sim-gate deploy-after-sim verify-api verify-browser verify-graphiti-local

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

package-companion-beta: build-companion
	cd companion && bun run build:tauri:beta
	mkdir -p $(COMPANION_BETA_DIST)
	tools/verify-companion-beta-bundle companion/src-tauri/target/release/bundle/macos/internkim.app
	hdiutil create -volname "internkim" -srcfolder companion/src-tauri/target/release/bundle/macos/internkim.app -ov -format UDZO "$(COMPANION_BETA_DIST)/$(COMPANION_BETA_MACOS_ARTIFACT)"
	@if [ -n "$$APPLE_SIGNING_IDENTITY" ]; then codesign --force --sign "$$APPLE_SIGNING_IDENTITY" "$(COMPANION_BETA_DIST)/$(COMPANION_BETA_MACOS_ARTIFACT)"; else echo "unsigned beta artifact: $(COMPANION_BETA_DIST)/$(COMPANION_BETA_MACOS_ARTIFACT)"; fi
	@if [ -n "$$APPLE_ID" ] && [ -n "$$APPLE_TEAM_ID" ] && [ -n "$$APPLE_APP_SPECIFIC_PASSWORD" ]; then xcrun notarytool submit "$(COMPANION_BETA_DIST)/$(COMPANION_BETA_MACOS_ARTIFACT)" --apple-id "$$APPLE_ID" --team-id "$$APPLE_TEAM_ID" --password "$$APPLE_APP_SPECIFIC_PASSWORD" --wait; fi

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

prepare-blueclaw-runtime-builder: build
	./internkim lab runtime-builder-prepare

prepare-blueclaw-runtime-base:
	if [ "$$(uname -s)" = "Linux" ]; then GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-runtime --builder local; else GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-runtime --builder tart; fi

prepare-blueclaw-payload:
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-payload

smoke-blueclaw-runtime-tart: build
	./internkim setup --sim --only blueclaw-runtime-base,blueclaw-payload,skills,services,users-sync --force-all --verify

smoke-blueclaw-runtime-tart-fast: build
	./internkim setup --sim --only binaries,blueclaw-runtime-base,blueclaw-payload,services --verify

build-litert-lm-main:
	tools/build-litert-lm-main

deps-graphiti:
	cd .dependency/blueclaw && test -x .venv-graphiti/bin/python || uv venv .venv-graphiti
	cd .dependency/blueclaw && uv pip install --python .venv-graphiti/bin/python -r tools/graphiti_memoryd/requirements.txt

setup-sim: build
	./internkim setup --sim

sim-gate: build
	./internkim sim gate

deploy-after-sim: build
	./internkim update --sim-first

verify-api: build
	./internkim verify api
	./internkim verify mattermost

verify-browser: build
	./internkim verify browser --local
	./internkim verify browser --public

verify-graphiti-local:
	tools/verify-graphiti-local
