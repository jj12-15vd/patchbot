.PHONY: setup build install test demo

PREFIX ?= $(HOME)/.local

setup:
	npm ci --prefix analyzer --ignore-scripts --no-audit --no-fund
	go mod download

# Keep the helper beside the executable; do not distribute just the Go binary.
build:
	mkdir -p dist/analyzer
	go build -o dist/patchbot ./cmd/patchbot
	cp -R analyzer/src dist/analyzer/
	cp analyzer/package.json analyzer/package-lock.json dist/analyzer/
	npm ci --prefix dist/analyzer --ignore-scripts --no-audit --no-fund

install: build
	sh scripts/install.sh "$(PREFIX)"

test:
	go test ./...
	npm test --prefix analyzer

# Findings are expected: the demo exits 1, not 0. Do not hide code 2 failures.
demo:
	mkdir -p dist
	go build -o dist/demo ./cmd/demo
	./dist/demo; code=$$?; test $$code -eq 1
