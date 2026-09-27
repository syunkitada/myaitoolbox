BINARY := myntfy

.PHONY: all build test lint tidy clean

all: build

build:
	go build -o $(BINARY) ./cmd/myntfy

test:
	go test ./...

lint:
	golangci-lint run

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)
