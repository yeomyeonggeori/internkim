GO_HOST ?= $(shell go env GOOS 2>/dev/null)-$(shell go env GOARCH 2>/dev/null)
GO_CACHE ?= /tmp/internkim-go-cache-$(GO_HOST)
GO_MOD_CACHE ?= /tmp/internkim-go-mod-cache-$(GO_HOST)
BLUECLAW_GO_CACHE ?= /tmp/blueclaw-go-cache
COMPANION_TARGET_TRIPLE ?= $(shell rustc -vV 2>/dev/null | sed -n 's/host: //p')
AGENT_BROWSER_VERSION ?= 0.26.0
COMPANION_BETA_DIST ?= dist/companion
COMPANION_BETA_MACOS_ARTIFACT ?= internkim-companion-beta-macos-aarch64.dmg

.PHONY: build build-companion build-companion-shell package-companion-beta build-mattermost-ephemeral-plugin verify-generated-protocol check test doctor deps-sim deps-browser deps-companion deps-companion-browser prepare-blueclaw-runtime-builder prepare-blueclaw-runtime-base prepare-blueclaw-payload prepare-blueclaw-llmd smoke-blueclaw-runtime-lab smoke-blueclaw-runtime-lab-fast deps-graphiti setup-sim fleet-gate deploy-after-fleet sim-gate deploy-after-sim verify-api verify-browser verify-graphiti-local

build: verify-generated-protocol build-mattermost-ephemeral-plugin
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o internkim ./cmd/internkim
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go build -o internkim-llm-gateway ./cmd/internkim-llm-gateway

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

build-mattermost-ephemeral-plugin:
	cd mattermost-plugin/internkim-ephemeral && GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) GOOS=linux GOARCH=amd64 go build -o dist/plugin-linux-amd64 ./
	cd mattermost-plugin/internkim-ephemeral && GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) GOOS=linux GOARCH=arm64 go build -o dist/plugin-linux-arm64 ./
	rm -rf build/mattermost-plugins/.package-com.internkim.ephemeral
	mkdir -p build/mattermost-plugins/.package-com.internkim.ephemeral/com.internkim.ephemeral/server/dist
	cp mattermost-plugin/internkim-ephemeral/plugin.json build/mattermost-plugins/.package-com.internkim.ephemeral/com.internkim.ephemeral/plugin.json
	cp mattermost-plugin/internkim-ephemeral/dist/plugin-linux-amd64 build/mattermost-plugins/.package-com.internkim.ephemeral/com.internkim.ephemeral/server/dist/plugin-linux-amd64
	cp mattermost-plugin/internkim-ephemeral/dist/plugin-linux-arm64 build/mattermost-plugins/.package-com.internkim.ephemeral/com.internkim.ephemeral/server/dist/plugin-linux-arm64
	mkdir -p build/mattermost-plugins/.package-com.internkim.ephemeral/com.internkim.ephemeral/webapp/dist
	cp mattermost-plugin/internkim-ephemeral/webapp/main.js build/mattermost-plugins/.package-com.internkim.ephemeral/com.internkim.ephemeral/webapp/dist/main.js
	mkdir -p build/mattermost-plugins
	COPYFILE_DISABLE=1 tar --no-xattrs -czf build/mattermost-plugins/com.internkim.ephemeral-0.2.1.tar.gz -C build/mattermost-plugins/.package-com.internkim.ephemeral com.internkim.ephemeral
	rm -f build/mattermost-plugins/com.internkim.ephemeral-0.1.0.tar.gz
	rm -f build/mattermost-plugins/com.internkim.ephemeral-0.2.0.tar.gz
	rm -rf build/mattermost-plugins/.package-com.internkim.ephemeral

verify-generated-protocol:
	cd .dependency/blueclaw/protocol && bun install --frozen-lockfile
	cd .dependency/blueclaw/protocol && bun run generate:check
	cd .dependency/blueclaw/protocol && bun run generate:check --target ../../../pkg/capabilityprotocol/generated

check: build build-companion
	cd companion && bun run check
	cd companion && bun run test
	cd web && bun run check
	cd web && bun run test:unit
	cd mattermost-plugin/internkim-ephemeral && bun test webapp
	cd .dependency/blueclaw/protocol && bun run build
	cd .dependency/blueclaw/protocol && bun test
	cd .dependency/blueclaw/llmd && bun install --frozen-lockfile
	cd .dependency/blueclaw/llmd && bun run build
	cd .dependency/blueclaw/llmd && bun test
	python3 -m unittest discover -s poc -p '*_test.py'
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) go test ./...
	cd .dependency/blueclaw && GOCACHE=$(BLUECLAW_GO_CACHE) go test ./...

test: check

doctor: build
	./internkim doctor

deps-sim:
	@echo "install the container CLI from https://github.com/apple/container/releases"

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
	if [ "$$(uname -s)" = "Linux" ]; then GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-runtime --builder local; else GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-runtime --builder container; fi

prepare-blueclaw-payload: verify-generated-protocol
	GOCACHE=$(GO_CACHE) GOMODCACHE=$(GO_MOD_CACHE) tools/prepare-blueclaw-payload

prepare-blueclaw-llmd: verify-generated-protocol
	tools/prepare-blueclaw-llmd

smoke-blueclaw-runtime-lab: build
	./internkim setup --sim --only blueclaw-runtime-base,blueclaw-payload,skills,services,users-sync --force-all --verify

smoke-blueclaw-runtime-lab-fast: build
	./internkim setup --sim --only binaries,blueclaw-runtime-base,blueclaw-payload,services --verify

build-litert-lm-main:
	tools/build-litert-lm-main

deps-graphiti:
	cd .dependency/blueclaw && test -x .venv-graphiti/bin/python || uv venv .venv-graphiti
	cd .dependency/blueclaw && uv pip install --python .venv-graphiti/bin/python -r tools/graphiti_memoryd/requirements.txt

setup-sim: build
	./internkim setup --sim

fleet-gate: build
	./internkim dev fleet run

deploy-after-fleet: build
	./internkim dev fleet run
	./internkim deploy

sim-gate: fleet-gate

deploy-after-sim: deploy-after-fleet

verify-api: build
	./internkim verify api
	./internkim verify mattermost

verify-browser: build
	./internkim verify browser --local
	./internkim verify browser --public

verify-graphiti-local:
	tools/verify-graphiti-local
