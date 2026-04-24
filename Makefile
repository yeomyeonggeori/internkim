GO_CACHE ?= /tmp/internkim-go-cache
BLUECLAW_GO_CACHE ?= /tmp/blueclaw-go-cache

.PHONY: build check test doctor deps-sim setup-sim

build:
	GOCACHE=$(GO_CACHE) go build -o internkim ./cmd/internkim

check: build
	cd web && npm run check
	GOCACHE=$(GO_CACHE) go test ./...
	cd .dependency/blueclaw && GOCACHE=$(BLUECLAW_GO_CACHE) go test ./...

test: check

doctor: build
	./internkim doctor

deps-sim:
	brew install cirruslabs/cli/tart

setup-sim: build
	./internkim setup --sim
