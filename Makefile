BINARY_NAME=vault-secrets-cleanup
BIN_DIR=bin

.PHONY: build test clean proto

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/vault-secrets-cleanup

test:
	go test ./...

proto:
	protoc --go_out=. --go_opt=paths=source_relative pkg/proto/*.proto

clean:
	rm -rf $(BIN_DIR)
