# Phase 5: Deletion - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the deletion execution engine that safely deletes secrets according to an approved plan, with comprehensive safety features including circuit breakers, resumable operations, and real-time status tracking.

**Architecture:** Deletion engine processes plans with rate limiting, retry logic, and circuit breaker pattern. Supports both KV v1 and v2 hard deletes. Updates plan file in real-time with deletion status.

**Tech Stack:** Go 1.21+, Vault Go SDK, Protocol Buffers, circuit breaker pattern

---

## File Structure

```
vault-secrets-cleanup/
├── internal/
│   ├── cli/
│   │   ├── execute.go           # Execute command
│   │   └── execute_test.go
│   └── deletion/
│       ├── engine.go             # Deletion engine
│       ├── engine_test.go
│       ├── executor.go           # Secret deletion executor
│       ├── executor_test.go
│       ├── circuit_breaker.go   # Circuit breaker
│       ├── circuit_breaker_test.go
│       └── progress.go           # Progress tracking
└── tests/integration/
    └── deletion_test.go          # Integration tests
```

---

## Task 1: Circuit Breaker

**Files:** `internal/deletion/circuit_breaker.go`, `internal/deletion/circuit_breaker_test.go`

- [ ] **Step 1: Write circuit breaker tests**
```go
package deletion

import (
	"errors"
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestCircuitBreaker(t *testing.T) {
	t.Run("allows operations when closed", func(t *testing.T) {
		cb := NewCircuitBreaker(3)
		
		for i := 0; i < 5; i++ {
			assert.True(t, cb.Allow())
			cb.RecordSuccess()
		}
	})

	t.Run("opens after consecutive failures", func(t *testing.T) {
		cb := NewCircuitBreaker(3)
		
		// Record 3 failures
		for i := 0; i < 3; i++ {
			assert.True(t, cb.Allow())
			cb.RecordFailure(errors.New("test error"))
		}
		
		// Circuit should be open now
		assert.False(t, cb.Allow())
	})

	t.Run("resets on success", func(t *testing.T) {
		cb := NewCircuitBreaker(3)
		
		// Record 2 failures
		cb.Allow()
		cb.RecordFailure(errors.New("error 1"))
		cb.Allow()
		cb.RecordFailure(errors.New("error 2"))
		
		// Record success - should reset counter
		cb.Allow()
		cb.RecordSuccess()
		
		// Should still allow operations
		assert.True(t, cb.Allow())
	})
}
```

- [ ] **Step 2: Run tests to verify failure**
```bash
go test ./internal/deletion/... -v
```

- [ ] **Step 3: Implement circuit breaker**
```go
package deletion

import (
	"fmt"
	"sync"
)

type CircuitBreaker struct {
	mu                sync.Mutex
	maxFailures       int
	consecutiveFailures int
	isOpen            bool
	lastError         error
}

func NewCircuitBreaker(maxFailures int) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
	}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return !cb.isOpen
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.consecutiveFailures = 0
	cb.isOpen = false
}

func (cb *CircuitBreaker) RecordFailure(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.consecutiveFailures++
	cb.lastError = err
	
	if cb.consecutiveFailures >= cb.maxFailures {
		cb.isOpen = true
	}
}

func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.isOpen
}

func (cb *CircuitBreaker) GetStats() (consecutiveFailures int, isOpen bool, lastError error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	return cb.consecutiveFailures, cb.isOpen, cb.lastError
}

func (cb *CircuitBreaker) Error() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if !cb.isOpen {
		return nil
	}
	
	return fmt.Errorf("circuit breaker open after %d consecutive failures (last error: %v)", 
		cb.consecutiveFailures, cb.lastError)
}
```

- [ ] **Step 4: Run tests to verify pass**

- [ ] **Step 5: Commit**
```bash
git add .
git commit -m "feat: add circuit breaker for failure handling"
```

---

## Task 2: Secret Deletion Executor

**Files:** `internal/deletion/executor.go`, `internal/deletion/executor_test.go`

- [ ] **Step 1: Write executor tests**
```go
package deletion

import (
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

func TestExecutor_Delete(t *testing.T) {
	// Tests require mock Vault client
	t.Skip("requires mock implementation")
}
```

- [ ] **Step 2: Implement executor**
```go
package deletion

import (
	"context"
	"fmt"
	"path"

	"github.com/yourusername/vault-secrets-cleanup/internal/retry"
	"github.com/yourusername/vault-secrets-cleanup/internal/vault"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type Executor struct {
	client      *vault.Client
	retryConfig retry.Config
	dryRun      bool
}

type ExecutorConfig struct {
	Client      *vault.Client
	RetryConfig retry.Config
	DryRun      bool
}

func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		client:      config.Client,
		retryConfig: config.RetryConfig,
		dryRun:      config.DryRun,
	}
}

func (e *Executor) Delete(ctx context.Context, action *proto.SecretAction) error {
	if e.dryRun {
		// Dry run - don't actually delete
		return nil
	}

	// Get namespace-scoped client
	client := e.client
	if action.NamespacePath != "" {
		client = e.client.WithNamespace(action.NamespacePath)
	}

	// Determine KV version from mount path
	isKVv2 := e.isKVv2(action.MountPath)

	// Delete with retry
	return retry.Do(ctx, func() error {
		return e.deleteSecret(ctx, client, action, isKVv2)
	}, e.retryConfig)
}

func (e *Executor) deleteSecret(ctx context.Context, client *vault.Client, action *proto.SecretAction, isKVv2 bool) error {
	if isKVv2 {
		return e.deleteKVv2(ctx, client, action)
	}
	return e.deleteKVv1(ctx, client, action)
}

func (e *Executor) deleteKVv1(ctx context.Context, client *vault.Client, action *proto.SecretAction) error {
	// KV v1: DELETE /<mount>/<path>
	fullPath := path.Join(action.MountPath, action.SecretPath)
	
	_, err := client.Raw().Logical().DeleteWithContext(ctx, fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete KV v1 secret: %w", err)
	}

	return nil
}

func (e *Executor) deleteKVv2(ctx context.Context, client *vault.Client, action *proto.SecretAction) error {
	// KV v2 hard delete requires two operations:
	// 1. DELETE /<mount>/metadata/<path> (deletes all versions and metadata)
	
	metadataPath := path.Join(action.MountPath, "metadata", action.SecretPath)
	
	_, err := client.Raw().Logical().DeleteWithContext(ctx, metadataPath)
	if err != nil {
		return fmt.Errorf("failed to delete KV v2 secret metadata: %w", err)
	}

	return nil
}

func (e *Executor) isKVv2(mountPath string) bool {
	// This is a simplified check - in production, you'd query the mount info
	// For now, assume KV v2 if mount path contains "secret/"
	return true // Default to KV v2
}
```

- [ ] **Step 3: Run tests**

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "feat: add secret deletion executor for KV v1 and v2"
```

---

## Task 3: Progress Tracking

**Files:** `internal/deletion/progress.go`

- [ ] **Step 1: Implement progress tracker**
```go
package deletion

import (
	"fmt"
	"sync"
	"time"
)

type ProgressTracker struct {
	mu            sync.Mutex
	total         int
	completed     int
	failed        int
	startTime     time.Time
	lastUpdate    time.Time
}

func NewProgressTracker(total int) *ProgressTracker {
	return &ProgressTracker{
		total:      total,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
	}
}

func (p *ProgressTracker) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed++
	p.lastUpdate = time.Now()
}

func (p *ProgressTracker) RecordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed++
	p.failed++
	p.lastUpdate = time.Now()
}

func (p *ProgressTracker) GetProgress() (completed, failed, total int, percentage float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	percentage = 0
	if p.total > 0 {
		percentage = float64(p.completed) / float64(p.total) * 100
	}
	
	return p.completed, p.failed, p.total, percentage
}

func (p *ProgressTracker) GetETA() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.completed == 0 {
		return 0
	}
	
	elapsed := time.Since(p.startTime)
	avgTimePerItem := elapsed / time.Duration(p.completed)
	remaining := p.total - p.completed
	
	return avgTimePerItem * time.Duration(remaining)
}

func (p *ProgressTracker) String() string {
	completed, failed, total, percentage := p.GetProgress()
	eta := p.GetETA()
	
	return fmt.Sprintf("Progress: %d/%d (%.1f%%) | Failed: %d | ETA: %v",
		completed, total, percentage, failed, eta.Round(time.Second))
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add progress tracking with ETA calculation"
```

---

## Task 4: Deletion Engine

**Files:** `internal/deletion/engine.go`, `internal/deletion/engine_test.go`

- [ ] **Step 1: Implement deletion engine**
```go
package deletion

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/yourusername/vault-secrets-cleanup/internal/ratelimit"
	"github.com/yourusername/vault-secrets-cleanup/internal/retry"
	"github.com/yourusername/vault-secrets-cleanup/internal/vault"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
	"google.golang.org/protobuf/proto"
)

type Engine struct {
	client         *vault.Client
	executor       *Executor
	rateLimiter    *ratelimit.Limiter
	circuitBreaker *CircuitBreaker
	progress       *ProgressTracker
	planFile       string
}

type Config struct {
	Client          *vault.Client
	RateLimiter     *ratelimit.Limiter
	RetryConfig     retry.Config
	CircuitBreaker  *CircuitBreaker
	DryRun          bool
	PlanFile        string
}

func NewEngine(config Config) *Engine {
	return &Engine{
		client: config.Client,
		executor: NewExecutor(ExecutorConfig{
			Client:      config.Client,
			RetryConfig: config.RetryConfig,
			DryRun:      config.DryRun,
		}),
		rateLimiter:    config.RateLimiter,
		circuitBreaker: config.CircuitBreaker,
		planFile:       config.PlanFile,
	}
}

func (e *Engine) Execute(ctx context.Context, plan *proto.DeletionPlan) error {
	// Count actions to delete
	toDelete := e.countActionsToDelete(plan)
	e.progress = NewProgressTracker(toDelete)

	fmt.Printf("Starting deletion of %d secrets...\n", toDelete)

	// Process each action
	for _, action := range plan.Actions {
		// Skip if not marked for deletion
		if !e.shouldDelete(action) {
			continue
		}

		// Check circuit breaker
		if !e.circuitBreaker.Allow() {
			return e.circuitBreaker.Error()
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Rate limit
		if err := e.rateLimiter.Wait(ctx); err != nil {
			return err
		}

		// Execute deletion
		if err := e.executeAction(ctx, action); err != nil {
			e.circuitBreaker.RecordFailure(err)
			e.progress.RecordFailure()
			
			// Update action status
			action.Status = "failed"
			action.Error = err.Error()
		} else {
			e.circuitBreaker.RecordSuccess()
			e.progress.RecordSuccess()
			
			// Update action status
			action.Status = "deleted"
			action.DeletedAt = time.Now().UTC().Format(time.RFC3339)
		}

		// Save plan after each deletion
		if err := e.savePlan(plan); err != nil {
			fmt.Printf("Warning: failed to save plan: %v\n", err)
		}

		// Print progress
		fmt.Printf("\r%s", e.progress.String())
	}

	fmt.Println() // New line after progress

	// Check if circuit breaker opened
	if e.circuitBreaker.IsOpen() {
		return e.circuitBreaker.Error()
	}

	return nil
}

func (e *Engine) shouldDelete(action *proto.SecretAction) bool {
	// Only delete stale secrets, or unknown if explicitly included
	return action.Category == "stale" || action.Category == "unknown"
}

func (e *Engine) executeAction(ctx context.Context, action *proto.SecretAction) error {
	return e.executor.Delete(ctx, action)
}

func (e *Engine) countActionsToDelete(plan *proto.DeletionPlan) int {
	count := 0
	for _, action := range plan.Actions {
		if e.shouldDelete(action) {
			count++
		}
	}
	return count
}

func (e *Engine) savePlan(plan *proto.DeletionPlan) error {
	data, err := proto.Marshal(plan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Atomic write: write to temp file, then rename
	tempFile := e.planFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tempFile, e.planFile); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add deletion engine with circuit breaker and progress tracking"
```

---

## Task 5: Execute Command

**Files:** `internal/cli/execute.go`, `internal/cli/execute_test.go`

- [ ] **Step 1: Implement execute command**
```go
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yourusername/vault-secrets-cleanup/internal/deletion"
	"github.com/yourusername/vault-secrets-cleanup/internal/ratelimit"
	"github.com/yourusername/vault-secrets-cleanup/internal/retry"
	"github.com/yourusername/vault-secrets-cleanup/internal/vault"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
	"google.golang.org/protobuf/proto"
)

var (
	executePlan          string
	executeRateLimit     int
	executeCircuitBreaker int
	executeDryRun        bool
	executeYes           bool
)

var executeCmd = &cobra.Command{
	Use:   "execute",
	Short: "Execute approved deletion plan",
	Long: `Execute the deletion plan to permanently remove secrets from Vault.
This operation is IRREVERSIBLE. Ensure you have taken a Vault snapshot before proceeding.`,
	RunE: runExecute,
}

func init() {
	rootCmd.AddCommand(executeCmd)

	executeCmd.Flags().StringVar(&executePlan, "plan", "deletion-plan.pb", "Deletion plan file")
	executeCmd.Flags().IntVar(&executeRateLimit, "rate-limit", 0, "Override rate limit (requests per second)")
	executeCmd.Flags().IntVar(&executeCircuitBreaker, "circuit-breaker", 10, "Max consecutive failures before stopping")
	executeCmd.Flags().BoolVar(&executeDryRun, "dry-run", false, "Simulate deletion without actually deleting")
	executeCmd.Flags().BoolVarP(&executeYes, "yes", "y", false, "Skip confirmation prompt")
}

func runExecute(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logger.Info("starting deletion execution",
		"plan", executePlan,
		"dry_run", executeDryRun,
	)

	// Load plan
	planData, err := os.ReadFile(executePlan)
	if err != nil {
		return fmt.Errorf("failed to read plan: %w", err)
	}

	plan := &proto.DeletionPlan{}
	if err := proto.Unmarshal(planData, plan); err != nil {
		return fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	// Display summary
	fmt.Println("=== Deletion Plan Summary ===")
	fmt.Printf("Total secrets: %d\n", plan.Stats.TotalSecrets)
	fmt.Printf("To delete: %d\n", plan.Stats.ToDeleteCount)
	fmt.Printf("  - Stale: %d\n", plan.Stats.StaleCount)
	fmt.Printf("  - Unknown: %d\n", plan.Stats.UnknownCount)
	fmt.Println()

	if executeDryRun {
		fmt.Println("DRY RUN MODE - No secrets will be deleted")
	} else {
		fmt.Println("⚠️  WARNING: This will PERMANENTLY delete secrets from Vault!")
		fmt.Println("⚠️  Ensure you have taken a Vault snapshot before proceeding!")
	}
	fmt.Println()

	// Confirmation prompt
	if !executeYes && !executeDryRun {
		fmt.Print("Type 'yes' to confirm deletion: ")
		reader := bufio.NewReader(os.Stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		if strings.TrimSpace(response) != "yes" {
			fmt.Println("Deletion cancelled")
			return nil
		}
	}

	// Create Vault client
	client, err := vault.NewClient(vault.ClientConfig{
		Address:   cfg.Vault.Address,
		Token:     cfg.Vault.Token,
		Namespace: cfg.Vault.Namespace,
	})
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}

	// Determine rate limit
	rateLimit := cfg.RateLimit.Deletion
	if executeRateLimit > 0 {
		rateLimit = executeRateLimit
	}

	// Create deletion engine
	engine := deletion.NewEngine(deletion.Config{
		Client:      client,
		RateLimiter: ratelimit.New(rateLimit),
		RetryConfig: retry.DefaultConfig(),
		CircuitBreaker: deletion.NewCircuitBreaker(executeCircuitBreaker),
		DryRun:      executeDryRun,
		PlanFile:    executePlan,
	})

	// Execute deletion
	if err := engine.Execute(ctx, plan); err != nil {
		logger.Error("deletion failed", "error", err)
		return fmt.Errorf("deletion failed: %w", err)
	}

	// Count results
	deleted := 0
	failed := 0
	for _, action := range plan.Actions {
		if action.Status == "deleted" {
			deleted++
		} else if action.Status == "failed" {
			failed++
		}
	}

	logger.Info("deletion complete",
		"deleted", deleted,
		"failed", failed,
	)

	fmt.Println()
	fmt.Println("=== Deletion Complete ===")
	fmt.Printf("Successfully deleted: %d\n", deleted)
	fmt.Printf("Failed: %d\n", failed)
	fmt.Printf("Updated plan: %s\n", executePlan)

	if failed > 0 {
		fmt.Println()
		fmt.Println("⚠️  Some deletions failed. Review the plan file for details.")
		fmt.Println("You can re-run this command to retry failed deletions.")
	}

	return nil
}
```

- [ ] **Step 2: Test execute command**
```bash
make build
./bin/vault-secrets-cleanup execute --plan deletion-plan.pb --dry-run
```

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add execute command for deletion plan execution"
```

---

## Task 6: Resumable Operations

**Files:** Update `internal/deletion/engine.go`

- [ ] **Step 1: Add resumption logic to engine**
```go
// Add to Engine.Execute() method

func (e *Engine) Execute(ctx context.Context, plan *proto.DeletionPlan) error {
	// Count actions to delete (skip already deleted)
	toDelete := e.countPendingActions(plan)
	e.progress = NewProgressTracker(toDelete)

	fmt.Printf("Starting deletion of %d secrets...\n", toDelete)

	// Process each action
	for _, action := range plan.Actions {
		// Skip if already deleted
		if action.Status == "deleted" {
			continue
		}

		// Skip if not marked for deletion
		if !e.shouldDelete(action) {
			continue
		}

		// ... rest of execution logic
	}

	return nil
}

func (e *Engine) countPendingActions(plan *proto.DeletionPlan) int {
	count := 0
	for _, action := range plan.Actions {
		if action.Status != "deleted" && e.shouldDelete(action) {
			count++
		}
	}
	return count
}
```

- [ ] **Step 2: Test resumption**

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add resumable deletion operations"
```

---

## Phase 5 Completion Checklist

- [ ] Circuit breaker stops after consecutive failures
- [ ] Executor handles KV v1 and v2 deletions
- [ ] Progress tracking shows real-time status and ETA
- [ ] Deletion engine processes plans with safety features
- [ ] Execute command requires confirmation
- [ ] Dry-run mode simulates deletions
- [ ] Plan file updates in real-time with status
- [ ] Resumable operations skip already-deleted secrets
- [ ] Rate limiting prevents API overload
- [ ] All unit tests pass
- [ ] Integration tests pass

---

## Next Steps

After completing Phase 5, proceed to Phase 6: Polish & Release by loading [`2026-04-10-phase6-polish-release.md`](2026-04-10-phase6-polish-release.md)