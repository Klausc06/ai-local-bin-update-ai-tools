SHELL := /usr/bin/env bash

BINARY := update-ai-tools
DIST_DIR := dist
GOCACHE ?= $(CURDIR)/.gocache
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.1.0-dev)
LDFLAGS := -s -w -X 'update-ai-tools/internal/app.version=$(VERSION)'

.PHONY: test build release install clean

test:
	GOCACHE="$(GOCACHE)" go test -race ./...

build:
	mkdir -p "$(DIST_DIR)"
	GOCACHE="$(GOCACHE)" go build -ldflags "$(LDFLAGS)" -trimpath -o "$(DIST_DIR)/$(BINARY)" ./cmd/update-ai-tools

release:
	GOCACHE="$(GOCACHE)" VERSION="$(VERSION)" LDFLAGS="$(LDFLAGS)" scripts/build-release.sh

install: build
	mkdir -p "$$HOME/.local/bin"
	cp "$(DIST_DIR)/$(BINARY)" "$$HOME/.local/bin/$(BINARY)"
	chmod +x "$$HOME/.local/bin/$(BINARY)"
	"$$HOME/.local/bin/$(BINARY)" --version

clean:
	rm -rf "$(DIST_DIR)" "$(GOCACHE)"
