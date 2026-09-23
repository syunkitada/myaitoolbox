BINARY := mcpctl
VERSION ?= dev

.PHONY: all build test lint tidy clean

all: build

build:
	go build -o $(BINARY) ./cmd/mcpctl

test:
	go test ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)