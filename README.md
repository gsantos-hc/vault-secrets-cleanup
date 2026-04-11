# Vault Secrets Cleanup

A Go-based CLI tool for Vault Enterprise that helps identify and remove stale static secrets from KV engines based on access patterns derived from audit logs.

## ⚠️ CRITICAL WARNING

**This tool performs PERMANENT, IRREVERSIBLE deletions of secrets from Vault.**

Before running any deletion operations:

1. **TAKE A VAULT SNAPSHOT** - Use Vault's snapshot functionality to create a backup
2. **TEST IN NON-PRODUCTION** - Validate the tool's behavior in a test environment first
3. **REVIEW DELETION PLANS** - Always review generated plans before executing
4. **USE DRY-RUN MODE** - Test your configuration with `--dry-run` before actual deletions

**The tool does NOT create backups.** Recovery from accidental deletions is only possible if you have taken Vault snapshots beforehand.

## Overview

This tool helps DevOps and Platform teams maintain Vault hygiene by:

- Discovering all secrets across namespaces and KV mounts
- Analyzing audit logs to determine last access timestamps
- Identifying stale secrets that haven't been accessed in a configurable period
- Generating deletion plans for review and approval
- Executing approved deletions with safety features

**Target Scale**: Large deployments (1-10GB+ audit logs, tens of millions of entries)

## Features

- **Comprehensive Discovery**: Enumerate all namespaces, KV mounts, and secrets
- **Audit Log Analysis**: Stream and parse large audit logs efficiently (10GB+ in <30 minutes)
- **Flexible Staleness Criteria**: Configure different retention periods per namespace
- **Safety Features**: Dry-run mode, confirmation prompts, circuit breakers, exclusion rules
- **Performance**: Rate limiting, concurrent processing, streaming for large datasets
- **Protocol Buffers**: Efficient binary format for large inventories (3-10x smaller than JSON)
- **Resumable Operations**: Continue from checkpoints after interruptions
- **Detailed Reporting**: Generate reports in multiple formats (JSON, Markdown, CSV)

## Quick Start

```bash
# 1. Take a Vault snapshot
vault operator raft snapshot save backup.snap

# 2. Validate setup
vault-secrets-cleanup validate \
  --check-permissions \
  --check-connectivity

# 3. Discover secrets
vault-secrets-cleanup discover \
  --output inventory.pb

# 4. Analyze audit logs
vault-secrets-cleanup analyze \
  --audit-log "/var/log/vault/audit-*.log" \
  --inventory inventory.pb \
  --output access.pb

# 5. Create deletion plan
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-period 365d \
  --output deletion-plan.pb

# 6. Review plan
vault-secrets-cleanup report \
  --plan deletion-plan.pb \
  --format markdown

# 7. Execute (after review and snapshot!)
vault-secrets-cleanup execute \
  --plan deletion-plan.pb \
  --rate-limit 5
```

## Cluster Seeding Script

Use the standalone seeding script to generate synthetic Vault Enterprise data with:

- configurable namespace count,
- randomized mount distribution per namespace,
- randomized secret distribution per mount,
- exact user-defined total secret count.

```bash
go run ./scripts/seed \
  --config example-config.yaml \
  --namespaces 25 \
  --total-secrets 50000 \
  --kv2-probability 0.9 \
  --namespace-prefix seed \
  --mount-prefix seed \
  --seed 42
```

Dry run example (no writes):

```bash
go run ./scripts/seed \
  --config example-config.yaml \
  --namespaces 10 \
  --total-secrets 1000 \
  --dry-run
```

Required Vault capabilities for seeding include namespace creation, mount creation, and secret writes:

```hcl
path "sys/namespaces/*" {
  capabilities = ["create", "update"]
}

path "+/sys/mounts/*" {
  capabilities = ["create", "update"]
}

path "+/*/data/*" {
  capabilities = ["create", "update"]
}

path "+/*" {
  capabilities = ["create", "update"]
}
```

## Documentation

- **[REQUIREMENTS.md](REQUIREMENTS.md)** - Complete requirements specification
- **[SUMMARY.md](SUMMARY.md)** - Executive summary and key recommendations
- **[docs/installation.md](docs/installation.md)** - Installation and build options
- **[docs/configuration.md](docs/configuration.md)** - Full configuration reference
- **[docs/usage.md](docs/usage.md)** - CLI command usage and examples
- **[docs/workflows.md](docs/workflows.md)** - End-to-end operational workflows
- **[docs/troubleshooting.md](docs/troubleshooting.md)** - Common issues and fixes
- **[docs/examples](docs/examples)** - Practical cleanup scenarios

## Requirements

- Go 1.21+
- Vault 1.11+ (Enterprise for namespace support)
- Vault token with appropriate permissions (see [REQUIREMENTS.md](REQUIREMENTS.md))
- 2GB RAM minimum (4GB recommended)
- 10GB disk space for large operations

## Installation

```bash
# Clone repository
git clone https://github.com/gsantos-hc/vault-secrets-cleanup
cd vault-secrets-cleanup

# Build
go build -o vault-secrets-cleanup ./cmd/vault-secrets-cleanup

# Install
go install ./cmd/vault-secrets-cleanup
```

## Configuration

Create `~/.vault-secrets-cleanup.yaml`:

```yaml
vault:
  address: https://vault.example.com
  auth_method: token
  token: ${VAULT_TOKEN}

rate_limit:
  discovery: 100
  deletion: 10

staleness:
  default_period: 365d
  namespaces:
    prod/*: 730    # 2 years for production
    dev/*: 90      # 90 days for dev

exclusions:
  namespaces:
    - admin/*
    - system/*
  paths:
    - "*/bootstrap/*"
    - "*/root-token"
```

## Safety Features

1. **Dry-run mode**: Generate plans without executing
2. **Confirmation prompts**: Require explicit approval before deletion
3. **Circuit breaker**: Stop after N consecutive failures
4. **Exclusion rules**: Protect critical secrets
5. **Unknown-access opt-in**: Require explicit flag to delete secrets with no audit trail
6. **Resumable operations**: Continue after interruption
7. **Hard delete**: Permanently remove secrets (KV v1 and v2)

## Vault Permissions Required

### Discovery Operations
```hcl
# List namespaces
path "sys/namespaces" {
  capabilities = ["list"]
}

# List mounts in all namespaces
path "+/sys/mounts" {
  capabilities = ["read"]
}

# List secrets in all KV mounts
path "+/secret/metadata/*" {
  capabilities = ["list"]
}
```

### Deletion Operations
```hcl
# Delete secrets in all KV v2 mounts
path "+/secret/data/*" {
  capabilities = ["delete"]
}

path "+/secret/metadata/*" {
  capabilities = ["delete"]
}

# Delete secrets in all KV v1 mounts
path "+/kv/*" {
  capabilities = ["delete"]
}
```

## Contributing

Contributions are welcome! Please read the requirements documentation and follow Go best practices.

## Testing and Release Readiness

```bash
# Unit tests
go test ./...

# Integration tests (requires local test Vault)
./scripts/setup-test-vault.sh

# In another shell
export VAULT_TEST_ADDR=http://127.0.0.1:8200
export VAULT_TEST_TOKEN=root
go test -tags=integration -v ./tests/integration/...

# End-to-end workflow test
go test -v ./tests/e2e/...

# Benchmarks
./scripts/benchmark.sh
```

CI is defined in [ci.yml](.github/workflows/ci.yml) and release automation in [release.yml](.github/workflows/release.yml).

### Release Tag Policy

Release tags are tool-generated by `commit-and-tag-version` from conventional commits on `main`.

- Do not create release tags manually.
- The release workflow computes the next version, updates `CHANGELOG.md`, creates a release commit, and pushes the corresponding `v*` tag.
- Build artifacts and GitHub release publication run for the generated tag in the same pipeline.

Version bump behavior follows conventional commits through the release tool:

- Breaking changes trigger a major version bump.
- Feature commits trigger a minor version bump.
- Fix commits trigger a patch version bump.

## License

[Your License Here]

## Support

For issues and questions, please open a GitHub issue.

---

**Remember**: Always take Vault snapshots before running deletions. This tool performs permanent operations and does not create backups.
