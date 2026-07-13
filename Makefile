APP=vpn-bypass
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/vpn-bypass

test:
	go test ./...

clean:
	rm -rf bin dist
