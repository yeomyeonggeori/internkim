GO_CACHE ?= /tmp/internkim-go-cache
BLUECLAW_GO_CACHE ?= /tmp/blueclaw-go-cache

.PHONY: build check test doctor deps-sim deps-browser setup-sim verify-api verify-browser

build:
	GOCACHE=$(GO_CACHE) go build -o internkim ./cmd/internkim

check: build
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

setup-sim: build
	./internkim setup --sim

verify-api: build
	./internkim verify api
	./internkim verify mattermost

verify-browser: build
	./internkim verify browser --local
	./internkim verify browser --public
