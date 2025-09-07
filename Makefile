SHELL := /bin/bash
MODULE := github.com/amirimatin/go-cluster

.PHONY: all build test lint tidy vet fmt

all: build

build:
	go build ./...

test:
	go test ./...

test-integration:
	go test -tags=integration ./integration -v

vet:
	go vet ./...

fmt:
	@gofmt -s -w .

tidy:
	go mod tidy

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
	  golangci-lint run; \
	else \
	  echo "golangci-lint not installed; skipping. (install: https://golangci-lint.run)"; \
	fi
