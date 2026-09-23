BINARY := mybox
VERSION ?= dev

.PHONY: all build test lint generate web-build web-dev tidy clean e2e

all: build

build:
	go build -o $(BINARY) ./cmd/mybox

test:
	go test ./...

e2e:
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

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
