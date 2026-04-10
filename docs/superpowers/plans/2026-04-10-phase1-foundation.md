# Phase 1: Foundation - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the foundational infrastructure for the Vault Secrets Cleanup tool including CLI framework, configuration management, Vault client, rate limiting, and retry logic.

**Architecture:** Modular design with clear separation of concerns. CLI layer uses Cobra/Viper, service layer provides reusable components (Vault client, rate limiter, retry logic), and infrastructure layer handles low-level operations.

**Tech Stack:** Go 1.21+, Cobra (CLI), Viper (config), Vault Go SDK, golang.org/x/time/rate (rate limiting)

---

## File Structure

```
vault-secrets-cleanup/
├── cmd/vault-secrets-cleanup/
│   └── main.go                    # Entry point
├── internal/
│   ├── cli/
│   │   ├── root.go               # Root command
│   │   ├── validate.go           # Validate command
│   │   └── validate_test.go
│   ├── config/
│   │   ├── config.go             # Configuration management
│   │   └── config_test.go
│   ├── vault/
│   │   ├── client.go             # Vault client wrapper
│   │   └── client_test.go
│   ├── ratelimit/
│   │   ├── limiter.go            # Rate limiter
│   │   └── limiter_test.go
│   ├── retry/
│   │   ├── retry.go              # Retry logic
│   │   └── retry_test.go
│   └── logging/
│       ├── logger.go             # Structured logging
│       └── logger_test.go
├── go.mod
├── go.sum
├── Makefile
├── .gitignore
└── example-config.yaml
```

---

## Task 1: Project Setup

**Files:** `go.mod`, `cmd/vault-secrets-cleanup/main.go`, `internal/cli/root.go`, `Makefile`, `.gitignore`

- [ ] **Step 1: Initialize Go module**
```bash
go mod init github.com/gsantos-hc/vault-secrets-cleanup
```

- [ ] **Step 2: Create main.go**
```go
package main

import (
	"os"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Create root command** (see [`internal/cli/root.go`](internal/cli/root.go:1) in detailed plan)

- [ ] **Step 4: Add dependencies**
```bash
go get github.com/spf13/cobra@latest
go get github.com/spf13/viper@latest
go get github.com/hashicorp/vault/api@latest
go mod tidy
```

- [ ] **Step 5: Create Makefile** with build, test, clean targets

- [ ] **Step 6: Create .gitignore** for binaries, coverage, IDE files

- [ ] **Step 7: Test build**
```bash
make build
./bin/vault-secrets-cleanup --help
```

- [ ] **Step 8: Commit**
```bash
git add .
git commit -m "feat: initialize project structure and CLI framework"
```

---

## Task 2: Configuration Management

**Files:** `internal/config/config.go`, `internal/config/config_test.go`, `example-config.yaml`

**Key Features:**
- Three-tier config: file → env vars → CLI flags
- YAML format with Viper
- Validation of required fields
- Support for VAULT_ADDR and VAULT_TOKEN env vars

- [ ] **Step 1: Write configuration tests** (TDD approach)

- [ ] **Step 2: Run tests to verify failure**
```bash
go test ./internal/config/... -v
```

- [ ] **Step 3: Implement Config struct** with VaultConfig, RateLimitConfig, StalenessConfig, ExclusionsConfig, LoggingConfig

- [ ] **Step 4: Implement Load() function** with Viper integration

- [ ] **Step 5: Implement Validate() method** for required fields

- [ ] **Step 6: Add testify dependency**
```bash
go get github.com/stretchr/testify@latest
go mod tidy
```

- [ ] **Step 7: Run tests to verify pass**
```bash
go test ./internal/config/... -v
```

- [ ] **Step 8: Create example-config.yaml** with all options documented

- [ ] **Step 9: Integrate into CLI root command**

- [ ] **Step 10: Test config loading**
```bash
./bin/vault-secrets-cleanup --config example-config.yaml --help
```

- [ ] **Step 11: Commit**
```bash
git add .
git commit -m "feat: add configuration management with file, env, and flag support"
```

---

## Task 3: Vault Client Wrapper

**Files:** `internal/vault/client.go`, `internal/vault/client_test.go`

**Key Features:**
- Wraps Vault API client
- Health check support
- Namespace enumeration
- Token renewal
- Per-namespace client creation

- [ ] **Step 1: Write client tests**

- [ ] **Step 2: Run tests to verify failure**

- [ ] **Step 3: Implement Client struct** with api.Client wrapper

- [ ] **Step 4: Implement NewClient()** with config validation

- [ ] **Step 5: Implement Health()** for connectivity checks

- [ ] **Step 6: Implement ListNamespaces()** for discovery

- [ ] **Step 7: Implement RenewToken()** and StartTokenRenewal()

- [ ] **Step 8: Implement WithNamespace()** for namespace-scoped clients

- [ ] **Step 9: Run tests to verify pass**

- [ ] **Step 10: Add integration test** (requires running Vault)

- [ ] **Step 11: Commit**
```bash
git add .
git commit -m "feat: add Vault client wrapper with health check and namespace support"
```

---

## Task 4: Rate Limiter

**Files:** `internal/ratelimit/limiter.go`, `internal/ratelimit/limiter_test.go`

**Key Features:**
- Wraps golang.org/x/time/rate
- Configurable requests per second
- Burst support
- Context-aware waiting

- [ ] **Step 1: Write rate limiter tests**

- [ ] **Step 2: Run tests to verify failure**

- [ ] **Step 3: Implement Limiter struct** wrapping rate.Limiter

- [ ] **Step 4: Implement New()** and NewWithBurst()

- [ ] **Step 5: Implement Wait()** with context support

- [ ] **Step 6: Implement Allow()**, SetLimit(), SetBurst()

- [ ] **Step 7: Add dependency**
```bash
go get golang.org/x/time/rate@latest
go mod tidy
```

- [ ] **Step 8: Run tests to verify pass**

- [ ] **Step 9: Commit**
```bash
git add .
git commit -m "feat: add rate limiter with configurable requests per second"
```

---

## Task 5: Retry Logic

**Files:** `internal/retry/retry.go`, `internal/retry/retry_test.go`

**Key Features:**
- Exponential backoff with jitter
- Configurable max attempts
- Context cancellation support
- Retryable error detection

- [ ] **Step 1: Write retry tests**

- [ ] **Step 2: Run tests to verify failure**

- [ ] **Step 3: Implement Config struct** with MaxAttempts, InitialDelay, MaxDelay, Multiplier

- [ ] **Step 4: Implement DefaultConfig()**

- [ ] **Step 5: Implement Do()** with exponential backoff

- [ ] **Step 6: Implement IsRetryable()** for error classification

- [ ] **Step 7: Implement DoWithRetryable()** for selective retry

- [ ] **Step 8: Run tests to verify pass**

- [ ] **Step 9: Commit**
```bash
git add .
git commit -m "feat: add retry logic with exponential backoff and jitter"
```

---

## Task 6: Logging Infrastructure

**Files:** `internal/logging/logger.go`, `internal/logging/logger_test.go`

**Key Features:**
- Wraps log/slog
- Text and JSON formats
- Configurable log levels
- Structured logging with key-value pairs

- [ ] **Step 1: Write logger tests**

- [ ] **Step 2: Run tests to verify failure**

- [ ] **Step 3: Implement Logger struct** wrapping slog.Logger

- [ ] **Step 4: Implement New()** with Config (Level, Format, Output)

- [ ] **Step 5: Implement Debug(), Info(), Warn(), Error()**

- [ ] **Step 6: Implement With()** and WithGroup()** for context

- [ ] **Step 7: Run tests to verify pass**

- [ ] **Step 8: Integrate into CLI root command**

- [ ] **Step 9: Test logging**
```bash
export VAULT_ADDR=https://vault.example.com
export VAULT_TOKEN=test-token
./bin/vault-secrets-cleanup --config example-config.yaml --help
```

- [ ] **Step 10: Commit**
```bash
git add .
git commit -m "feat: add structured logging with text and JSON formats"
```

---

## Task 7: Validate Command

**Files:** `internal/cli/validate.go`, `internal/cli/validate_test.go`

**Key Features:**
- Connectivity check (health endpoint)
- Permission validation (list namespaces)
- Clear success/failure output

- [ ] **Step 1: Write validate command tests**

- [ ] **Step 2: Run tests to verify failure**

- [ ] **Step 3: Implement validateCmd** with flags

- [ ] **Step 4: Implement runValidate()** function

- [ ] **Step 5: Implement validatePermissions()** helper

- [ ] **Step 6: Register command in root.go init()**

- [ ] **Step 7: Run tests to verify pass**

- [ ] **Step 8: Test validate command**
```bash
./bin/vault-secrets-cleanup validate --check-connectivity --check-permissions
```

- [ ] **Step 9: Commit**
```bash
git add .
git commit -m "feat: add validate command for connectivity and permissions"
```

---

## Phase 1 Completion Checklist

- [ ] All unit tests pass
- [ ] CLI builds without errors
- [ ] Configuration loads from file, env, and flags
- [ ] Vault client connects and authenticates
- [ ] Rate limiter prevents API overload
- [ ] Retry logic handles transient failures
- [ ] Logging outputs structured logs
- [ ] Validate command checks connectivity and permissions
- [ ] Code follows Go best practices
- [ ] All code is committed to git

---

## Next Steps

After completing Phase 1, proceed to Phase 2: Discovery by loading [`2026-04-10-phase2-discovery.md`](2026-04-10-phase2-discovery.md)