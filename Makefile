BINARY_NAME=vault-secrets-cleanup
BIN_DIR=bin

.PHONY: build test test-integration test-e2e bench clean proto

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/vault-secrets-cleanup

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
