# Installation

## Prerequisites

- Go 1.21+
- Vault 1.11+ (Enterprise for namespace support)
- Access token with permissions for discovery and cleanup

## Build From Source

```bash
git clone https://github.com/gsantos-hc/vault-secrets-cleanup
cd vault-secrets-cleanup
make build
```

Binary output is written to `bin/vault-secrets-cleanup`.

## Install With Go

```bash
go install ./cmd/vault-secrets-cleanup
```

## Verify Installation

```bash
vault-secrets-cleanup --help
```

## Local Development Setup

```bash
# Run unit tests
go test ./...

# Run integration tests (requires local Vault dev server)
./scripts/setup-test-vault.sh
```
