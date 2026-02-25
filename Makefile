.PHONY: build test test-e2e lint fmt check run clean

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X github.com/dwellir-public/cli/internal/cli.Version=$(VERSION) \
           -X github.com/dwellir-public/cli/internal/cli.Commit=$(COMMIT) \
           -X github.com/dwellir-public/cli/internal/cli.BuildDate=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/dwellir ./cmd/dwellir

test:
	go test ./...

test-e2e:
	go test ./test/e2e/ -tags=e2e -v

lint:
	golangci-lint run

fmt:
	goimports -w .

check: fmt lint test

run:
	go run ./cmd/dwellir

clean:
	rm -rf bin/
