BINARY := gosqlbackup
PKG := ./cmd/gosqlbackup
OUTPUT := bin/$(BINARY)
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X 'go-db-backup/internal/cli.Version=$(VERSION)'

.PHONY: all build test lint run clean tidy fmt

all: build

build:
	@mkdir -p bin
	go build -ldflags="$(LDFLAGS)" -o $(OUTPUT) $(PKG)

test:
	go test ./...

test-verbose:
	go test -v ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

tidy:
	go mod tidy

run: build
	./$(OUTPUT)

clean:
	rm -rf bin/ backups/
