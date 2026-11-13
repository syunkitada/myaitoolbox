BINARY := myherdr

.PHONY: all build test lint clean

all: build

build:
	go build -o $(BINARY) ./cmd/myherdr

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -f $(BINARY)
