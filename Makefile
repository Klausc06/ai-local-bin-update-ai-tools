SHELL := /usr/bin/env bash

BINARY := update-ai-tools
DIST_DIR := dist
GOCACHE ?= $(CURDIR)/.gocache

.PHONY: test build release install clean

test:
	GOCACHE="$(GOCACHE)" go test ./...

build:
	mkdir -p "$(DIST_DIR)"
	GOCACHE="$(GOCACHE)" go build -trimpath -o "$(DIST_DIR)/$(BINARY)" ./cmd/update-ai-tools

release:
	GOCACHE="$(GOCACHE)" scripts/build-release.sh

install: build
	mkdir -p "$$HOME/.local/bin"
	cp "$(DIST_DIR)/$(BINARY)" "$$HOME/.local/bin/$(BINARY)"
	chmod +x "$$HOME/.local/bin/$(BINARY)"
	"$$HOME/.local/bin/$(BINARY)" --version

clean:
	rm -rf "$(DIST_DIR)" "$(GOCACHE)"
