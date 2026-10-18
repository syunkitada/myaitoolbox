BINARY := mybox
VERSION ?= dev
GOBIN ?= $(shell go env GOBIN)
ifeq ($(strip $(GOBIN)),)
override GOBIN := $(shell go env GOBIN)
ifeq ($(strip $(GOBIN)),)
override GOBIN := $(firstword $(subst :, ,$(shell go env GOPATH)))/bin
endif
endif
BROWSER_WORKER_INSTALL_DIR := $(GOBIN)/browser-worker

.PHONY: all build install test test-install lint generate web-build web-dev browser-worker-install tidy clean e2e

all: build

build:
	go build -o $(BINARY) ./cmd/mybox

install: web-build
	@case "$(GOBIN)" in /*) ;; *) printf 'GOBIN must be an absolute path: %s\n' "$(GOBIN)" >&2; exit 1 ;; esac
	@if [ -L "$(BROWSER_WORKER_INSTALL_DIR)" ] || { [ -e "$(BROWSER_WORKER_INSTALL_DIR)" ] && [ ! -d "$(BROWSER_WORKER_INSTALL_DIR)" ]; }; then \
		printf 'refusing to install browser worker into non-directory or symlink: %s\n' "$(BROWSER_WORKER_INSTALL_DIR)" >&2; exit 1; \
	fi
	@if [ -d "$(BROWSER_WORKER_INSTALL_DIR)" ] && [ -n "$$(find "$(BROWSER_WORKER_INSTALL_DIR)" -mindepth 1 -maxdepth 1 -print -quit)" ] && { [ ! -f "$(BROWSER_WORKER_INSTALL_DIR)/package.json" ] || ! grep -Eq '"name"[[:space:]]*:[[:space:]]*"mybox-browser-worker"' "$(BROWSER_WORKER_INSTALL_DIR)/package.json"; }; then \
		printf 'refusing to overwrite an unrelated directory: %s\n' "$(BROWSER_WORKER_INSTALL_DIR)" >&2; exit 1; \
	fi
	@if [ -L "$(BROWSER_WORKER_INSTALL_DIR)/src" ] || [ -L "$(BROWSER_WORKER_INSTALL_DIR)/package.json" ] || [ -L "$(BROWSER_WORKER_INSTALL_DIR)/package-lock.json" ] || [ -L "$(BROWSER_WORKER_INSTALL_DIR)/node_modules" ] || { [ -d "$(BROWSER_WORKER_INSTALL_DIR)/src" ] && [ -n "$$(find "$(BROWSER_WORKER_INSTALL_DIR)/src" -type l -print -quit)" ]; }; then \
		printf 'refusing to update browser worker paths containing symlinks: %s\n' "$(BROWSER_WORKER_INSTALL_DIR)" >&2; exit 1; \
	fi
	@mkdir -p "$(BROWSER_WORKER_INSTALL_DIR)"
	@cp -R browser-worker/src browser-worker/package.json browser-worker/package-lock.json "$(BROWSER_WORKER_INSTALL_DIR)/"
	@cd "$(BROWSER_WORKER_INSTALL_DIR)" && npm ci && npx playwright install chromium
	@GOBIN="$(GOBIN)" go install ./cmd/mybox
	@printf 'Installed mybox and browser worker in %s\n' "$(GOBIN)"

test: test-install
	go test ./...

test-install:
	bash ./tests/install_makefile_test.sh

e2e: browser-worker-install
	./tests/cli_e2e.sh
	cd web && npx playwright test

lint:
	golangci-lint run

generate:
	go generate ./...

web-build:
	cd web && npm install && npm run build
	mkdir -p internal/webui/dist
	find internal/webui/dist -mindepth 1 ! -name README.md ! -name .gitkeep -exec rm -rf {} +
	cp -r web/dist/. internal/webui/dist/
	touch internal/webui/dist/.gitkeep

web-dev:
	cd web && npm run dev

browser-worker-install:
	cd browser-worker && npm ci && npx playwright install chromium

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
