BINARY_NAME=vault-secrets-cleanup
BIN_DIR=bin
AUTO_VERSION=$(shell git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo v0.0.0-local)
VERSION ?= $(AUTO_VERSION)

.PHONY: build test test-integration test-e2e bench clean proto

build:
	@echo "Using version: $(VERSION)"
	mkdir -p $(BIN_DIR)
	go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/vault-secrets-cleanup

test:
	go test ./...

test-integration:
	go test -tags=integration -v ./tests/integration/...

test-e2e:
	go test -v ./tests/e2e/...

bench:
	./scripts/benchmark.sh

proto:
	protoc --go_out=. --go_opt=paths=source_relative pkg/proto/*.proto

clean:
	rm -rf $(BIN_DIR)
