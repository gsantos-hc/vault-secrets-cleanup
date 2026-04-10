# Phase 6: Polish & Release - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete end-to-end testing, performance optimization, security hardening, comprehensive documentation, and prepare for v1.0.0 release.

**Architecture:** Integration and E2E tests validate full workflows. CI/CD pipeline automates testing and releases. Documentation covers all use cases.

**Tech Stack:** Go 1.21+, GitHub Actions (CI/CD), golangci-lint, test Vault instance

---

## File Structure

```
vault-secrets-cleanup/
├── .github/
│   └── workflows/
│       ├── ci.yml                # CI pipeline
│       └── release.yml           # Release pipeline
├── tests/
│   ├── integration/
│   │   ├── discovery_test.go
│   │   ├── audit_test.go
│   │   ├── planning_test.go
│   │   └── deletion_test.go
│   └── e2e/
│       └── full_workflow_test.go
├── docs/
│   ├── installation.md
│   ├── configuration.md
│   ├── usage.md
│   ├── workflows.md
│   ├── troubleshooting.md
│   └── examples/
│       ├── basic-cleanup.md
│       ├── external-import.md
│       └── incremental-cleanup.md
├── scripts/
│   ├── setup-test-vault.sh
│   └── benchmark.sh
└── CHANGELOG.md
```

---

## Task 1: Integration Tests

**Files:** `tests/integration/*_test.go`

- [ ] **Step 1: Setup test Vault script**
```bash
#!/bin/bash
# scripts/setup-test-vault.sh

set -e

echo "Starting Vault in dev mode..."
vault server -dev -dev-root-token-id=root &
VAULT_PID=$!

sleep 2

export VAULT_ADDR='http://127.0.0.1:8200'
export VAULT_TOKEN='root'

echo "Creating test namespaces..."
vault namespace create prod
vault namespace create dev

echo "Enabling KV engines..."
vault secrets enable -path=secret kv-v2
vault secrets enable -path=kv kv

echo "Creating test secrets..."
vault kv put secret/app1/config username=admin password=secret
vault kv put secret/app2/config api_key=12345

echo "Test Vault ready at $VAULT_ADDR"
echo "Root token: root"
echo "PID: $VAULT_PID"
```

- [ ] **Step 2: Write integration test for discovery**
```go
package integration

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yourusername/vault-secrets-cleanup/internal/discovery"
	"github.com/yourusername/vault-secrets-cleanup/internal/ratelimit"
	"github.com/yourusername/vault-secrets-cleanup/internal/vault"
)

func TestDiscovery_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	addr := os.Getenv("VAULT_ADDR")
	token := os.Getenv("VAULT_TOKEN")
	if addr == "" || token == "" {
		t.Skip("VAULT_ADDR and VAULT_TOKEN required")
	}

	client, err := vault.NewClient(vault.ClientConfig{
		Address: addr,
		Token:   token,
	})
	require.NoError(t, err)

	engine := discovery.NewEngine(discovery.Config{
		Client:      client,
		RateLimiter: ratelimit.New(100),
		Workers:     5,
	})

	inventory, err := engine.Discover(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, inventory)
	assert.Greater(t, inventory.Stats.SecretCount, int32(0))
}
```

- [ ] **Step 3: Write integration tests for other components**

- [ ] **Step 4: Run integration tests**
```bash
./scripts/setup-test-vault.sh
go test -v ./tests/integration/... -timeout 5m
```

- [ ] **Step 5: Commit**
```bash
git add .
git commit -m "test: add integration tests for all components"
```

---

## Task 2: End-to-End Tests

**Files:** `tests/e2e/full_workflow_test.go`

- [ ] **Step 1: Write E2E test**
```go
package e2e

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test")
	}

	// This test validates the complete workflow:
	// 1. Discover secrets
	// 2. Analyze audit logs (or import access data)
	// 3. Generate deletion plan
	// 4. Execute plan (dry-run)

	t.Run("discover", func(t *testing.T) {
		// Test discovery command
	})

	t.Run("analyze", func(t *testing.T) {
		// Test analyze command
	})

	t.Run("plan", func(t *testing.T) {
		// Test plan command
	})

	t.Run("execute_dry_run", func(t *testing.T) {
		// Test execute command in dry-run mode
	})
}
```

- [ ] **Step 2: Implement E2E test**

- [ ] **Step 3: Run E2E tests**
```bash
go test -v ./tests/e2e/... -timeout 10m
```

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "test: add end-to-end workflow tests"
```

---

## Task 3: Performance Benchmarks

**Files:** `scripts/benchmark.sh`, benchmark tests

- [ ] **Step 1: Create benchmark script**
```bash
#!/bin/bash
# scripts/benchmark.sh

set -e

echo "Running performance benchmarks..."

# Benchmark discovery
echo "Benchmarking discovery..."
time ./bin/vault-secrets-cleanup discover --output bench-inventory.pb

# Benchmark audit processing (with sample 1GB log)
echo "Benchmarking audit processing..."
time ./bin/vault-secrets-cleanup analyze \
  --audit-log sample-audit-1gb.log \
  --output bench-access.pb

# Benchmark plan generation
echo "Benchmarking plan generation..."
time ./bin/vault-secrets-cleanup plan \
  --inventory bench-inventory.pb \
  --access-data bench-access.pb \
  --output bench-plan.pb

echo "Benchmarks complete"
```

- [ ] **Step 2: Write benchmark tests**
```go
package discovery

import (
	"context"
	"testing"
)

func BenchmarkDiscovery(b *testing.B) {
	// Benchmark discovery performance
}

func BenchmarkAuditParsing(b *testing.B) {
	// Benchmark audit log parsing
}
```

- [ ] **Step 3: Run benchmarks**
```bash
go test -bench=. -benchmem ./...
```

- [ ] **Step 4: Optimize based on results**

- [ ] **Step 5: Commit**
```bash
git add .
git commit -m "perf: add performance benchmarks and optimizations"
```

---

## Task 4: Security Audit

**Files:** Security documentation, code review

- [ ] **Step 1: Run security linters**
```bash
# Install gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run security scan
gosec ./...
```

- [ ] **Step 2: Review credential handling**
- Ensure VAULT_TOKEN never logged
- Verify no secrets in error messages
- Check file permissions on output files

- [ ] **Step 3: Review permission requirements**
- Document minimum required Vault policies
- Test with least-privilege token

- [ ] **Step 4: Add security documentation**

- [ ] **Step 5: Commit**
```bash
git add .
git commit -m "security: complete security audit and hardening"
```

---

## Task 5: Documentation

**Files:** `docs/*.md`, `README.md` updates

- [ ] **Step 1: Write installation guide**
```markdown
# Installation

## Prerequisites
- Go 1.21+
- Vault 1.11+ (Enterprise for namespace support)
- 2GB RAM minimum (4GB recommended)

## Install from source
\`\`\`bash
git clone https://github.com/yourusername/vault-secrets-cleanup
cd vault-secrets-cleanup
make install
\`\`\`

## Install from release
\`\`\`bash
# Download latest release
curl -LO https://github.com/yourusername/vault-secrets-cleanup/releases/latest/download/vault-secrets-cleanup-linux-amd64

# Make executable
chmod +x vault-secrets-cleanup-linux-amd64
mv vault-secrets-cleanup-linux-amd64 /usr/local/bin/vault-secrets-cleanup
\`\`\`
```

- [ ] **Step 2: Write configuration guide**

- [ ] **Step 3: Write usage guide with examples**

- [ ] **Step 4: Write troubleshooting guide**

- [ ] **Step 5: Create example workflows**

- [ ] **Step 6: Update README.md**

- [ ] **Step 7: Commit**
```bash
git add .
git commit -m "docs: add comprehensive documentation"
```

---

## Task 6: CI/CD Pipeline

**Files:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`

- [ ] **Step 1: Create CI workflow**
```yaml
# .github/workflows/ci.yml
name: CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Install dependencies
        run: go mod download
      
      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
  
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run golangci-lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest
  
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build
        run: make build
```

- [ ] **Step 2: Create release workflow**
```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v4
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 3: Create .goreleaser.yml**
```yaml
# .goreleaser.yml
project_name: vault-secrets-cleanup

builds:
  - env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    main: ./cmd/vault-secrets-cleanup

archives:
  - format: tar.gz
    name_template: >-
      {{ .ProjectName }}_
      {{- .Version }}_
      {{- .Os }}_
      {{- .Arch }}

checksum:
  name_template: 'checksums.txt'

changelog:
  sort: asc
  filters:
    exclude:
      - '^docs:'
      - '^test:'
```

- [ ] **Step 4: Test CI pipeline**

- [ ] **Step 5: Commit**
```bash
git add .
git commit -m "ci: add CI/CD pipelines for testing and releases"
```

---

## Task 7: Release Preparation

**Files:** `CHANGELOG.md`, version tags

- [ ] **Step 1: Create CHANGELOG.md**
```markdown
# Changelog

All notable changes to this project will be documented in this file.

## [1.0.0] - 2024-XX-XX

### Added
- Initial release
- Secrets discovery across namespaces
- Audit log processing (10GB+ in <30 minutes)
- Configurable staleness criteria
- Exclusion rules with glob patterns
- Deletion plan generation
- Safe deletion execution with circuit breaker
- Protocol Buffer format for efficient storage
- Comprehensive CLI with validate, discover, analyze, plan, execute, report commands
- Dry-run mode for testing
- Resumable operations
- Progress tracking with ETA
- Multiple report formats (Markdown, JSON, CSV)

### Security
- Secure credential handling
- Permission validation
- No sensitive data in logs
```

- [ ] **Step 2: Update version in code**

- [ ] **Step 3: Create release checklist**
- [ ] All tests pass
- [ ] Documentation complete
- [ ] CHANGELOG updated
- [ ] Security audit complete
- [ ] Performance benchmarks meet targets
- [ ] Example configurations tested

- [ ] **Step 4: Create v1.0.0 tag**
```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

- [ ] **Step 5: Verify release artifacts**

- [ ] **Step 6: Commit**
```bash
git add .
git commit -m "release: prepare v1.0.0"
```

---

## Task 8: Post-Release Tasks

**Files:** GitHub release notes, announcements

- [ ] **Step 1: Create GitHub release**
- Upload binaries
- Add release notes
- Link to documentation

- [ ] **Step 2: Update README badges**
- Build status
- Test coverage
- Latest release

- [ ] **Step 3: Create announcement**

- [ ] **Step 4: Monitor for issues**

---

## Phase 6 Completion Checklist

- [ ] All integration tests pass
- [ ] End-to-end tests validate full workflow
- [ ] Performance benchmarks meet targets:
  - [ ] 10GB audit log in <30 minutes
  - [ ] Memory usage <500MB
  - [ ] Discovery handles 100k+ secrets
- [ ] Security audit complete
- [ ] Documentation comprehensive and clear
- [ ] CI/CD pipeline functional
- [ ] v1.0.0 released
- [ ] Release artifacts available
- [ ] GitHub release published

---

## Success Criteria

### Functional
- ✓ All requirements from REQUIREMENTS.md implemented
- ✓ All commands work as documented
- ✓ Error handling is robust

### Performance
- ✓ 10GB audit log processed in <30 minutes
- ✓ Memory usage <500MB for large operations
- ✓ Rate limiting prevents Vault overload

### Reliability
- ✓ No data corruption
- ✓ Idempotent operations
- ✓ Graceful error handling
- ✓ Resumable operations work correctly

### Security
- ✓ Secure credential handling
- ✓ Permission validation works
- ✓ No sensitive data in logs
- ✓ Audit trail of all actions

### Usability
- ✓ Clear CLI interface
- ✓ Comprehensive documentation
- ✓ Helpful error messages
- ✓ Example configurations work

---

## Post-v1.0 Roadmap

Future enhancements (Phase 7):
- Multi-cluster support
- Advanced analytics and reporting
- Notification system (Slack, Teams, webhooks)
- GitOps integration
- Metrics export (Prometheus)
- Dashboard generation
- Plugin system
- Cloud storage support

---

**Congratulations!** The Vault Secrets Cleanup tool is now ready for production use. 🎉