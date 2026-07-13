APP=vpn-bypass

.PHONY: build test clean

build:
	go build -o bin/$(APP) ./cmd/vpn-bypass

test:
	go test ./...

clean:
	rm -rf bin dist
