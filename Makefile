APP=vpn-bypass
VERSION ?= dev
COMMIT ?= $(shell git rev-parse HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

.PHONY: build test check clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/vpn-bypass

test:
	go test ./...

check:
	@test -z "$$(gofmt -l $$(git ls-files '*.go'))"
	go vet ./...
	go test ./...

clean:
	rm -rf bin dist
