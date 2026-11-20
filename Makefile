BINARY  := lead-sushi
PKG     := ./cmd/lead-sushi
BIN_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
ARGS    ?=

.DEFAULT_GOAL := help

.PHONY: help build run test vet fmt fmt-check check tidy install clean

help:
	@printf '%s\n' \
		'lead-sushi' \
		'' \
		'  make build        compile to $(BIN_DIR)/$(BINARY)' \
		'  make run ARGS=".." build, then run with flags, e.g. ARGS="--city sydney --niche clinics"' \
		'  make test         run tests with the race detector' \
		'  make vet          go vet' \
		'  make fmt          format all code with gofmt' \
		'  make fmt-check    fail if any file needs gofmt' \
		'  make check        fmt-check + vet + test (what CI runs)' \
		'  make tidy         go mod tidy' \
		'  make install      install the binary into GOBIN' \
		'  make clean        remove $(BIN_DIR)/'

build:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(BINARY) $(PKG)

run: build
	./$(BIN_DIR)/$(BINARY) $(ARGS)

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@unformatted="$$(gofmt -l .)"; \
	if [ -n "$$unformatted" ]; then \
		echo "gofmt needed on:"; echo "$$unformatted"; exit 1; \
	fi

check: fmt-check vet test

tidy:
	go mod tidy

install:
	go install $(LDFLAGS) $(PKG)

clean:
	rm -rf $(BIN_DIR)
