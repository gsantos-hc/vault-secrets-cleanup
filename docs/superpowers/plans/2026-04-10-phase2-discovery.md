# Phase 2: Discovery - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the discovery engine that enumerates all namespaces, KV mounts, and secrets across a Vault cluster, exporting the inventory in an efficient Protocol Buffer format.

**Architecture:** Discovery engine uses concurrent workers with rate limiting to enumerate Vault resources. Protocol Buffers provide efficient binary serialization for large inventories (3-10x smaller than JSON).

**Tech Stack:** Go 1.21+, Vault Go SDK, Protocol Buffers, concurrent workers with context cancellation

---

## File Structure

```
vault-secrets-cleanup/
├── pkg/proto/
│   ├── inventory.proto           # Protobuf schema
│   └── inventory.pb.go           # Generated code
├── internal/
│   ├── cli/
│   │   ├── discover.go           # Discover command
│   │   └── discover_test.go
│   └── discovery/
│       ├── engine.go             # Discovery engine
│       ├── engine_test.go
│       ├── namespace.go          # Namespace discovery
│       ├── mount.go              # Mount discovery
│       ├── secret.go             # Secret enumeration
│       └── progress.go           # Progress tracking
└── tests/integration/
    └── discovery_test.go         # Integration tests
```

---

## Task 1: Protocol Buffer Schema

**Files:** `pkg/proto/inventory.proto`, `pkg/proto/inventory.pb.go`

- [ ] **Step 1: Install protoc compiler**
```bash
# macOS
brew install protobuf

# Linux
apt-get install -y protobuf-compiler

# Verify
protoc --version
```

- [ ] **Step 2: Install Go protobuf plugin**
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
export PATH="$PATH:$(go env GOPATH)/bin"
```

- [ ] **Step 3: Create inventory.proto**
```protobuf
syntax = "proto3";

package proto;

option go_package = "github.com/yourusername/vault-secrets-cleanup/pkg/proto";

// Inventory represents a complete snapshot of secrets across Vault
message Inventory {
  string created_at = 1;           // ISO 8601 timestamp
  string vault_address = 2;        // Vault cluster address
  repeated Namespace namespaces = 3;
  InventoryStats stats = 4;
}

// Namespace represents a Vault namespace
message Namespace {
  string path = 1;                 // Namespace path (e.g., "prod/app1")
  string id = 2;                   // Namespace ID
  repeated Mount mounts = 3;
}

// Mount represents a KV mount within a namespace
message Mount {
  string path = 1;                 // Mount path (e.g., "secret/")
  string accessor = 2;             // Mount accessor
  string type = 3;                 // "kv" or "kv-v2"
  int32 version = 4;               // 1 or 2
  repeated Secret secrets = 5;
}

// Secret represents a secret path
message Secret {
  string path = 1;                 // Full secret path
  string discovered_at = 2;        // ISO 8601 timestamp
}

// InventoryStats contains summary statistics
message InventoryStats {
  int32 namespace_count = 1;
  int32 mount_count = 2;
  int32 secret_count = 3;
  int32 kv_v1_count = 4;
  int32 kv_v2_count = 5;
}
```

- [ ] **Step 4: Generate Go code**
```bash
protoc --go_out=. --go_opt=paths=source_relative pkg/proto/inventory.proto
```

- [ ] **Step 5: Add protobuf dependencies**
```bash
go get google.golang.org/protobuf@latest
go mod tidy
```

- [ ] **Step 6: Update Makefile**
```makefile
.PHONY: proto

proto:
	protoc --go_out=. --go_opt=paths=source_relative pkg/proto/*.proto
```

- [ ] **Step 7: Test protobuf generation**
```bash
make proto
go build ./pkg/proto/...
```

- [ ] **Step 8: Commit**
```bash
git add .
git commit -m "feat: add Protocol Buffer schema for inventory"
```

---

## Task 2: Discovery Engine Core

**Files:** `internal/discovery/engine.go`, `internal/discovery/engine_test.go`

- [ ] **Step 1: Write engine tests**
```go
package discovery

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_Discover(t *testing.T) {
	// Test will be implemented with mock Vault client
	t.Skip("requires mock implementation")
}
```

- [ ] **Step 2: Implement Engine struct**
```go
package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/yourusername/vault-secrets-cleanup/internal/ratelimit"
	"github.com/yourusername/vault-secrets-cleanup/internal/vault"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type Engine struct {
	client      *vault.Client
	rateLimiter *ratelimit.Limiter
	workers     int
	progress    *ProgressTracker
}

type Config struct {
	Client      *vault.Client
	RateLimiter *ratelimit.Limiter
	Workers     int
}

func NewEngine(config Config) *Engine {
	if config.Workers <= 0 {
		config.Workers = 10
	}

	return &Engine{
		client:      config.Client,
		rateLimiter: config.RateLimiter,
		workers:     config.Workers,
		progress:    NewProgressTracker(),
	}
}

func (e *Engine) Discover(ctx context.Context) (*proto.Inventory, error) {
	e.progress.Start()
	defer e.progress.Stop()

	inventory := &proto.Inventory{
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		VaultAddress: e.client.Raw().Address(),
		Namespaces:   []*proto.Namespace{},
		Stats:        &proto.InventoryStats{},
	}

	// Discover namespaces
	namespaces, err := e.discoverNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover namespaces: %w", err)
	}

	inventory.Namespaces = namespaces
	e.calculateStats(inventory)

	return inventory, nil
}

func (e *Engine) calculateStats(inventory *proto.Inventory) {
	stats := &proto.InventoryStats{}

	for _, ns := range inventory.Namespaces {
		stats.NamespaceCount++
		for _, mount := range ns.Mounts {
			stats.MountCount++
			stats.SecretCount += int32(len(mount.Secrets))
			if mount.Version == 1 {
				stats.KvV1Count += int32(len(mount.Secrets))
			} else {
				stats.KvV2Count += int32(len(mount.Secrets))
			}
		}
	}

	inventory.Stats = stats
}
```

- [ ] **Step 3: Run tests**
```bash
go test ./internal/discovery/... -v
```

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "feat: add discovery engine core structure"
```

---

## Task 3: Namespace Discovery

**Files:** `internal/discovery/namespace.go`

- [ ] **Step 1: Implement namespace discovery**
```go
package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

func (e *Engine) discoverNamespaces(ctx context.Context) ([]*proto.Namespace, error) {
	// Start with root namespace
	rootNS := &proto.Namespace{
		Path:   "",
		Id:     "root",
		Mounts: []*proto.Mount{},
	}

	// Discover mounts in root namespace
	mounts, err := e.discoverMounts(ctx, rootNS)
	if err != nil {
		return nil, fmt.Errorf("failed to discover root mounts: %w", err)
	}
	rootNS.Mounts = mounts

	namespaces := []*proto.Namespace{rootNS}

	// List child namespaces
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	childPaths, err := e.client.ListNamespaces(ctx)
	if err != nil {
		// If we can't list namespaces, just return root
		return namespaces, nil
	}

	// Discover each child namespace recursively
	for _, path := range childPaths {
		childNS, err := e.discoverNamespace(ctx, path)
		if err != nil {
			// Log error but continue with other namespaces
			e.progress.LogError(fmt.Sprintf("namespace %s: %v", path, err))
			continue
		}
		namespaces = append(namespaces, childNS)
	}

	return namespaces, nil
}

func (e *Engine) discoverNamespace(ctx context.Context, path string) (*proto.Namespace, error) {
	// Create namespace-scoped client
	nsClient := e.client.WithNamespace(path)

	ns := &proto.Namespace{
		Path:   path,
		Id:     extractNamespaceID(path),
		Mounts: []*proto.Mount{},
	}

	// Discover mounts in this namespace
	mounts, err := e.discoverMounts(ctx, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to discover mounts: %w", err)
	}
	ns.Mounts = mounts

	return ns, nil
}

func extractNamespaceID(path string) string {
	// In real implementation, this would query Vault for the actual ID
	// For now, use path as ID
	return strings.ReplaceAll(path, "/", "_")
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add namespace discovery with recursive enumeration"
```

---

## Task 4: Mount Discovery

**Files:** `internal/discovery/mount.go`

- [ ] **Step 1: Implement mount discovery**
```go
package discovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

func (e *Engine) discoverMounts(ctx context.Context, ns *proto.Namespace) ([]*proto.Mount, error) {
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}

	// Get namespace-scoped client
	nsClient := e.client
	if ns.Path != "" {
		nsClient = e.client.WithNamespace(ns.Path)
	}

	// List mounts
	mounts, err := nsClient.Raw().Sys().ListMounts()
	if err != nil {
		return nil, fmt.Errorf("failed to list mounts: %w", err)
	}

	var kvMounts []*proto.Mount

	for path, mount := range mounts {
		// Only process KV mounts
		if !isKVMount(mount.Type) {
			continue
		}

		kvMount := &proto.Mount{
			Path:     path,
			Accessor: mount.Accessor,
			Type:     mount.Type,
			Version:  getKVVersion(mount),
			Secrets:  []*proto.Secret{},
		}

		// Discover secrets in this mount
		secrets, err := e.discoverSecrets(ctx, ns, kvMount)
		if err != nil {
			e.progress.LogError(fmt.Sprintf("mount %s/%s: %v", ns.Path, path, err))
			continue
		}
		kvMount.Secrets = secrets

		kvMounts = append(kvMounts, kvMount)
	}

	return kvMounts, nil
}

func isKVMount(mountType string) bool {
	return mountType == "kv" || mountType == "generic"
}

func getKVVersion(mount *api.MountOutput) int32 {
	if mount.Options != nil {
		if version, ok := mount.Options["version"]; ok {
			if version == "2" {
				return 2
			}
		}
	}
	return 1
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add mount discovery for KV engines"
```

---

## Task 5: Secret Enumeration

**Files:** `internal/discovery/secret.go`

- [ ] **Step 1: Implement secret enumeration**
```go
package discovery

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

func (e *Engine) discoverSecrets(ctx context.Context, ns *proto.Namespace, mount *proto.Mount) ([]*proto.Secret, error) {
	nsClient := e.client
	if ns.Path != "" {
		nsClient = e.client.WithNamespace(ns.Path)
	}

	var secrets []*proto.Secret
	basePath := mount.Path

	// Adjust path for KV v2
	if mount.Version == 2 {
		basePath = path.Join(mount.Path, "metadata")
	}

	// Recursively list secrets
	err := e.listSecretsRecursive(ctx, nsClient, basePath, "", &secrets)
	if err != nil {
		return nil, err
	}

	e.progress.AddSecrets(len(secrets))
	return secrets, nil
}

func (e *Engine) listSecretsRecursive(ctx context.Context, client *vault.Client, basePath, subPath string, secrets *[]*proto.Secret) error {
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	fullPath := path.Join(basePath, subPath)
	
	secret, err := client.Raw().Logical().List(fullPath)
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", fullPath, err)
	}

	if secret == nil || secret.Data == nil {
		return nil
	}

	keys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil
	}

	for _, key := range keys {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}

		secretPath := path.Join(subPath, keyStr)

		// If key ends with /, it's a directory
		if strings.HasSuffix(keyStr, "/") {
			// Recurse into directory
			err := e.listSecretsRecursive(ctx, client, basePath, secretPath, secrets)
			if err != nil {
				e.progress.LogError(fmt.Sprintf("path %s: %v", secretPath, err))
				continue
			}
		} else {
			// It's a secret
			*secrets = append(*secrets, &proto.Secret{
				Path:         secretPath,
				DiscoveredAt: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	return nil
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add recursive secret enumeration for KV v1 and v2"
```

---

## Task 6: Progress Tracking

**Files:** `internal/discovery/progress.go`

- [ ] **Step 1: Implement progress tracker**
```go
package discovery

import (
	"fmt"
	"sync"
	"time"
)

type ProgressTracker struct {
	mu            sync.Mutex
	startTime     time.Time
	namespaces    int
	mounts        int
	secrets       int
	errors        []string
	running       bool
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		errors: []string{},
	}
}

func (p *ProgressTracker) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startTime = time.Now()
	p.running = true
}

func (p *ProgressTracker) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
}

func (p *ProgressTracker) AddNamespace() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.namespaces++
}

func (p *ProgressTracker) AddMount() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mounts++
}

func (p *ProgressTracker) AddSecrets(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.secrets += count
}

func (p *ProgressTracker) LogError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errors = append(p.errors, err)
}

func (p *ProgressTracker) Summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Since(p.startTime)
	
	summary := fmt.Sprintf(
		"Discovery complete in %v\n"+
		"  Namespaces: %d\n"+
		"  Mounts: %d\n"+
		"  Secrets: %d\n"+
		"  Errors: %d\n",
		elapsed.Round(time.Second),
		p.namespaces,
		p.mounts,
		p.secrets,
		len(p.errors),
	)

	if len(p.errors) > 0 {
		summary += "\nErrors:\n"
		for _, err := range p.errors {
			summary += fmt.Sprintf("  - %s\n", err)
		}
	}

	return summary
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add progress tracking for discovery operations"
```

---

## Task 7: Discover Command

**Files:** `internal/cli/discover.go`, `internal/cli/discover_test.go`

- [ ] **Step 1: Implement discover command**
```go
package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/vault-secrets-cleanup/internal/discovery"
	"github.com/yourusername/vault-secrets-cleanup/internal/ratelimit"
	"github.com/yourusername/vault-secrets-cleanup/internal/vault"
	"google.golang.org/protobuf/proto"
)

var (
	discoverOutput string
	discoverWorkers int
)

var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover all secrets across Vault namespaces",
	Long: `Enumerate all namespaces, KV mounts, and secrets in the Vault cluster.
Exports the inventory to a Protocol Buffer file for efficient storage.`,
	RunE: runDiscover,
}

func init() {
	rootCmd.AddCommand(discoverCmd)

	discoverCmd.Flags().StringVarP(&discoverOutput, "output", "o", "inventory.pb", "Output file for inventory")
	discoverCmd.Flags().IntVar(&discoverWorkers, "workers", 10, "Number of concurrent workers")
}

func runDiscover(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logger.Info("starting discovery",
		"output", discoverOutput,
		"workers", discoverWorkers,
	)

	// Create Vault client
	client, err := vault.NewClient(vault.ClientConfig{
		Address:   cfg.Vault.Address,
		Token:     cfg.Vault.Token,
		Namespace: cfg.Vault.Namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}

	// Create rate limiter
	rateLimiter := ratelimit.New(cfg.RateLimit.Discovery)

	// Create discovery engine
	engine := discovery.NewEngine(discovery.Config{
		Client:      client,
		RateLimiter: rateLimiter,
		Workers:     discoverWorkers,
	})

	// Run discovery
	inventory, err := engine.Discover(ctx)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	// Marshal to protobuf
	data, err := proto.Marshal(inventory)
	if err != nil {
		return fmt.Errorf("failed to marshal inventory: %w", err)
	}

	// Write to file
	if err := os.WriteFile(discoverOutput, data, 0644); err != nil {
		return fmt.Errorf("failed to write inventory: %w", err)
	}

	logger.Info("discovery complete",
		"namespaces", inventory.Stats.NamespaceCount,
		"mounts", inventory.Stats.MountCount,
		"secrets", inventory.Stats.SecretCount,
		"output", discoverOutput,
	)

	fmt.Printf("✓ Discovery complete\n")
	fmt.Printf("  Namespaces: %d\n", inventory.Stats.NamespaceCount)
	fmt.Printf("  Mounts: %d\n", inventory.Stats.MountCount)
	fmt.Printf("  Secrets: %d\n", inventory.Stats.SecretCount)
	fmt.Printf("  Output: %s\n", discoverOutput)

	return nil
}
```

- [ ] **Step 2: Test discover command**
```bash
make build
export VAULT_ADDR=https://vault.example.com
export VAULT_TOKEN=test-token
./bin/vault-secrets-cleanup discover --output test-inventory.pb
```

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add discover command for secrets inventory"
```

---

## Phase 2 Completion Checklist

- [ ] Protocol Buffer schema defined and generated
- [ ] Discovery engine discovers namespaces recursively
- [ ] Mount discovery identifies KV v1 and v2 mounts
- [ ] Secret enumeration handles large secret trees
- [ ] Progress tracking provides real-time feedback
- [ ] Discover command exports inventory to .pb file
- [ ] Rate limiting prevents API overload
- [ ] All unit tests pass
- [ ] Integration tests pass (with test Vault)
- [ ] Code follows Go best practices

---

## Next Steps

After completing Phase 2, proceed to Phase 3: Audit Processing by loading [`2026-04-10-phase3-audit-processing.md`](2026-04-10-phase3-audit-processing.md)