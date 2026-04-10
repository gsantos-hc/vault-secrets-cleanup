BINARY_NAME=vault-secrets-cleanup
BIN_DIR=bin

.PHONY: build test clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/vault-secrets-cleanup

test:
	go test ./...

clean:
	rm -rf $(BIN_DIR)
