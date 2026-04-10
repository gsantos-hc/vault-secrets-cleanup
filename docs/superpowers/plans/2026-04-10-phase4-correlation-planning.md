# Phase 4: Correlation & Planning - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the correlation engine that matches inventory with access data, applies staleness criteria and exclusion rules, and generates reviewable deletion plans.

**Architecture:** Correlation engine uses configurable matching strategies (strict vs path-based). Exclusion rules engine supports glob patterns. Plan generation creates Protocol Buffer files with categorized secrets.

**Tech Stack:** Go 1.21+, Protocol Buffers, glob pattern matching

---

## File Structure

```
vault-secrets-cleanup/
├── pkg/proto/
│   ├── plan.proto                # Protobuf schema for deletion plans
│   └── plan.pb.go                # Generated code
├── internal/
│   ├── cli/
│   │   ├── plan.go               # Plan command
│   │   ├── report.go             # Report command
│   │   └── plan_test.go
│   ├── correlation/
│   │   ├── engine.go             # Correlation engine
│   │   ├── engine_test.go
│   │   ├── matcher.go            # Matching strategies
│   │   └── matcher_test.go
│   ├── planning/
│   │   ├── planner.go            # Plan generation
│   │   ├── planner_test.go
│   │   ├── staleness.go          # Staleness calculation
│   │   └── exclusions.go         # Exclusion rules
│   └── reporting/
│       ├── generator.go          # Report generation
│       ├── markdown.go           # Markdown formatter
│       ├── json.go               # JSON formatter
│       └── csv.go                # CSV formatter
```

---

## Task 1: Deletion Plan Protocol Buffer Schema

**Files:** `pkg/proto/plan.proto`, `pkg/proto/plan.pb.go`

- [ ] **Step 1: Create plan.proto**
```protobuf
syntax = "proto3";

package proto;

option go_package = "github.com/yourusername/vault-secrets-cleanup/pkg/proto";

// DeletionPlan represents a plan for deleting stale secrets
message DeletionPlan {
  string created_at = 1;           // ISO 8601 timestamp
  string inventory_file = 2;       // Source inventory file
  string access_file = 3;          // Source access data file
  PlanConfig config = 4;           // Configuration used
  repeated SecretAction actions = 5;
  PlanStats stats = 6;
}

// PlanConfig contains the configuration used to generate the plan
message PlanConfig {
  int32 staleness_days = 1;
  string matching_strategy = 2;    // "strict" or "path-based"
  bool include_unknown = 3;
  repeated string exclusion_patterns = 4;
}

// SecretAction represents an action to take on a secret
message SecretAction {
  string namespace_path = 1;
  string namespace_id = 2;
  string mount_path = 3;
  string mount_accessor = 4;
  string secret_path = 5;
  string category = 6;             // "stale", "unknown", "active", "excluded"
  string reason = 7;               // Human-readable reason
  string last_accessed = 8;        // ISO 8601 timestamp (if known)
  int32 days_since_access = 9;     // Days since last access (if known)
  string status = 10;              // "pending", "deleted", "failed"
  string deleted_at = 11;          // ISO 8601 timestamp (if deleted)
  string error = 12;               // Error message (if failed)
}

// PlanStats contains summary statistics
message PlanStats {
  int32 total_secrets = 1;
  int32 stale_count = 2;
  int32 unknown_count = 3;
  int32 active_count = 4;
  int32 excluded_count = 5;
  int32 to_delete_count = 6;
}
```

- [ ] **Step 2: Generate Go code**
```bash
protoc --go_out=. --go_opt=paths=source_relative pkg/proto/plan.proto
make proto
```

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add Protocol Buffer schema for deletion plans"
```

---

## Task 2: Matching Strategies

**Files:** `internal/correlation/matcher.go`, `internal/correlation/matcher_test.go`

- [ ] **Step 1: Write matcher tests**
```go
package correlation

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

func TestMatcher_Match(t *testing.T) {
	inventory := &proto.Secret{Path: "myapp/config"}
	inventoryNS := &proto.Namespace{Path: "prod", Id: "ns123"}
	inventoryMount := &proto.Mount{Path: "secret/", Accessor: "kv_456"}

	access := &proto.AccessRecord{
		NamespacePath: "prod",
		NamespaceId:   "ns123",
		MountPath:     "secret/",
		MountAccessor: "kv_456",
		SecretPath:    "myapp/config",
	}

	t.Run("strict matching succeeds", func(t *testing.T) {
		matcher := NewMatcher(MatchingStrategyStrict)
		match := matcher.Match(inventory, inventoryNS, inventoryMount, access)
		assert.True(t, match)
	})

	t.Run("strict matching fails with different accessor", func(t *testing.T) {
		matcher := NewMatcher(MatchingStrategyStrict)
		differentAccess := *access
		differentAccess.MountAccessor = "kv_999"
		match := matcher.Match(inventory, inventoryNS, inventoryMount, &differentAccess)
		assert.False(t, match)
	})

	t.Run("path-based matching succeeds", func(t *testing.T) {
		matcher := NewMatcher(MatchingStrategyPathBased)
		differentAccess := *access
		differentAccess.NamespaceId = ""
		differentAccess.MountAccessor = ""
		match := matcher.Match(inventory, inventoryNS, inventoryMount, &differentAccess)
		assert.True(t, match)
	})
}
```

- [ ] **Step 2: Implement matching strategies**
```go
package correlation

import (
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type MatchingStrategy string

const (
	MatchingStrategyStrict    MatchingStrategy = "strict"
	MatchingStrategyPathBased MatchingStrategy = "path-based"
)

type Matcher struct {
	strategy MatchingStrategy
}

func NewMatcher(strategy MatchingStrategy) *Matcher {
	return &Matcher{strategy: strategy}
}

func (m *Matcher) Match(
	secret *proto.Secret,
	namespace *proto.Namespace,
	mount *proto.Mount,
	access *proto.AccessRecord,
) bool {
	switch m.strategy {
	case MatchingStrategyStrict:
		return m.strictMatch(secret, namespace, mount, access)
	case MatchingStrategyPathBased:
		return m.pathBasedMatch(secret, namespace, mount, access)
	default:
		return m.strictMatch(secret, namespace, mount, access)
	}
}

func (m *Matcher) strictMatch(
	secret *proto.Secret,
	namespace *proto.Namespace,
	mount *proto.Mount,
	access *proto.AccessRecord,
) bool {
	// Match by namespace ID + mount accessor + secret path
	return namespace.Id == access.NamespaceId &&
		mount.Accessor == access.MountAccessor &&
		secret.Path == access.SecretPath
}

func (m *Matcher) pathBasedMatch(
	secret *proto.Secret,
	namespace *proto.Namespace,
	mount *proto.Mount,
	access *proto.AccessRecord,
) bool {
	// Match by namespace path + mount path + secret path
	return namespace.Path == access.NamespacePath &&
		mount.Path == access.MountPath &&
		secret.Path == access.SecretPath
}
```

- [ ] **Step 3: Run tests to verify pass**

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "feat: add configurable matching strategies for correlation"
```

---

## Task 3: Correlation Engine

**Files:** `internal/correlation/engine.go`, `internal/correlation/engine_test.go`

- [ ] **Step 1: Implement correlation engine**
```go
package correlation

import (
	"fmt"
	"time"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type Engine struct {
	matcher *Matcher
}

type Config struct {
	MatchingStrategy MatchingStrategy
}

func NewEngine(config Config) *Engine {
	return &Engine{
		matcher: NewMatcher(config.MatchingStrategy),
	}
}

func (e *Engine) Correlate(inventory *proto.Inventory, accessData *proto.AccessData) (map[string]*proto.AccessRecord, error) {
	// Build access lookup map
	accessMap := make(map[string]*proto.AccessRecord)

	for _, ns := range inventory.Namespaces {
		for _, mount := range ns.Mounts {
			for _, secret := range mount.Secrets {
				// Find matching access record
				accessRecord := e.findAccessRecord(secret, ns, mount, accessData.Records)
				if accessRecord != nil {
					key := makeKey(ns, mount, secret)
					accessMap[key] = accessRecord
				}
			}
		}
	}

	return accessMap, nil
}

func (e *Engine) findAccessRecord(
	secret *proto.Secret,
	namespace *proto.Namespace,
	mount *proto.Mount,
	accessRecords []*proto.AccessRecord,
) *proto.AccessRecord {
	for _, access := range accessRecords {
		if e.matcher.Match(secret, namespace, mount, access) {
			return access
		}
	}
	return nil
}

func makeKey(ns *proto.Namespace, mount *proto.Mount, secret *proto.Secret) string {
	return fmt.Sprintf("%s|%s|%s", ns.Id, mount.Accessor, secret.Path)
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add correlation engine for inventory and access data"
```

---

## Task 4: Staleness Calculation

**Files:** `internal/planning/staleness.go`

- [ ] **Step 1: Implement staleness calculator**
```go
package planning

import (
	"fmt"
	"time"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type StalenessCalculator struct {
	defaultDays      int
	namespacePolicies map[string]int
}

type StalenessConfig struct {
	DefaultDays      int
	NamespacePolicies map[string]int
}

func NewStalenessCalculator(config StalenessConfig) *StalenessCalculator {
	return &StalenessCalculator{
		defaultDays:      config.DefaultDays,
		namespacePolicies: config.NamespacePolicies,
	}
}

func (s *StalenessCalculator) Calculate(
	secret *proto.Secret,
	namespace *proto.Namespace,
	mount *proto.Mount,
	access *proto.AccessRecord,
) (category string, reason string, daysSinceAccess int) {
	// Get staleness threshold for this namespace
	threshold := s.getThreshold(namespace.Path)

	// If no access record, it's unknown
	if access == nil {
		return "unknown", "No access record found in audit logs", -1
	}

	// Parse last accessed time
	lastAccessed, err := time.Parse(time.RFC3339, access.LastAccessed)
	if err != nil {
		return "unknown", fmt.Sprintf("Invalid timestamp: %s", access.LastAccessed), -1
	}

	// Calculate days since last access
	daysSinceAccess = int(time.Since(lastAccessed).Hours() / 24)

	// Determine category
	if daysSinceAccess > threshold {
		return "stale", fmt.Sprintf("Not accessed in %d days (threshold: %d)", daysSinceAccess, threshold), daysSinceAccess
	}

	return "active", fmt.Sprintf("Accessed %d days ago", daysSinceAccess), daysSinceAccess
}

func (s *StalenessCalculator) getThreshold(namespacePath string) int {
	// Check for exact match
	if threshold, ok := s.namespacePolicies[namespacePath]; ok {
		return threshold
	}

	// Check for wildcard matches
	for pattern, threshold := range s.namespacePolicies {
		if matchPattern(pattern, namespacePath) {
			return threshold
		}
	}

	return s.defaultDays
}

func matchPattern(pattern, path string) bool {
	// Simple wildcard matching (e.g., "prod/*" matches "prod/app1")
	if len(pattern) == 0 {
		return false
	}

	if pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(path) >= len(prefix) && path[:len(prefix)] == prefix
	}

	return pattern == path
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add staleness calculation with per-namespace policies"
```

---

## Task 5: Exclusion Rules Engine

**Files:** `internal/planning/exclusions.go`

- [ ] **Step 1: Implement exclusion rules**
```go
package planning

import (
	"path/filepath"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type ExclusionEngine struct {
	namespacePatterns []string
	pathPatterns      []string
}

type ExclusionConfig struct {
	NamespacePatterns []string
	PathPatterns      []string
}

func NewExclusionEngine(config ExclusionConfig) *ExclusionEngine {
	return &ExclusionEngine{
		namespacePatterns: config.NamespacePatterns,
		pathPatterns:      config.PathPatterns,
	}
}

func (e *ExclusionEngine) IsExcluded(
	secret *proto.Secret,
	namespace *proto.Namespace,
	mount *proto.Mount,
) (bool, string) {
	// Check namespace exclusions
	for _, pattern := range e.namespacePatterns {
		if matchGlob(pattern, namespace.Path) {
			return true, "Namespace matches exclusion pattern: " + pattern
		}
	}

	// Check path exclusions
	fullPath := mount.Path + secret.Path
	for _, pattern := range e.pathPatterns {
		if matchGlob(pattern, fullPath) {
			return true, "Path matches exclusion pattern: " + pattern
		}
	}

	return false, ""
}

func matchGlob(pattern, path string) bool {
	matched, err := filepath.Match(pattern, path)
	if err != nil {
		return false
	}
	return matched
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add exclusion rules engine with glob pattern support"
```

---

## Task 6: Plan Generation

**Files:** `internal/planning/planner.go`, `internal/planning/planner_test.go`

- [ ] **Step 1: Implement planner**
```go
package planning

import (
	"time"

	"github.com/yourusername/vault-secrets-cleanup/internal/correlation"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type Planner struct {
	stalenessCalc *StalenessCalculator
	exclusionEngine *ExclusionEngine
	includeUnknown bool
}

type Config struct {
	StalenessConfig StalenessConfig
	ExclusionConfig ExclusionConfig
	IncludeUnknown  bool
	MatchingStrategy correlation.MatchingStrategy
}

func NewPlanner(config Config) *Planner {
	return &Planner{
		stalenessCalc:   NewStalenessCalculator(config.StalenessConfig),
		exclusionEngine: NewExclusionEngine(config.ExclusionConfig),
		includeUnknown:  config.IncludeUnknown,
	}
}

func (p *Planner) GeneratePlan(
	inventory *proto.Inventory,
	accessData *proto.AccessData,
	accessMap map[string]*proto.AccessRecord,
	config *proto.PlanConfig,
) (*proto.DeletionPlan, error) {
	plan := &proto.DeletionPlan{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Config:    config,
		Actions:   []*proto.SecretAction{},
		Stats:     &proto.PlanStats{},
	}

	// Process each secret in inventory
	for _, ns := range inventory.Namespaces {
		for _, mount := range ns.Mounts {
			for _, secret := range mount.Secrets {
				action := p.processSecret(secret, ns, mount, accessMap)
				plan.Actions = append(plan.Actions, action)
				p.updateStats(plan.Stats, action)
			}
		}
	}

	return plan, nil
}

func (p *Planner) processSecret(
	secret *proto.Secret,
	namespace *proto.Namespace,
	mount *proto.Mount,
	accessMap map[string]*proto.AccessRecord,
) *proto.SecretAction {
	action := &proto.SecretAction{
		NamespacePath: namespace.Path,
		NamespaceId:   namespace.Id,
		MountPath:     mount.Path,
		MountAccessor: mount.Accessor,
		SecretPath:    secret.Path,
		Status:        "pending",
	}

	// Check exclusions first
	excluded, reason := p.exclusionEngine.IsExcluded(secret, namespace, mount)
	if excluded {
		action.Category = "excluded"
		action.Reason = reason
		return action
	}

	// Get access record
	key := correlation.makeKey(namespace, mount, secret)
	access := accessMap[key]

	// Calculate staleness
	category, reason, daysSinceAccess := p.stalenessCalc.Calculate(secret, namespace, mount, access)
	action.Category = category
	action.Reason = reason
	action.DaysSinceAccess = int32(daysSinceAccess)

	if access != nil {
		action.LastAccessed = access.LastAccessed
	}

	return action
}

func (p *Planner) updateStats(stats *proto.PlanStats, action *proto.SecretAction) {
	stats.TotalSecrets++

	switch action.Category {
	case "stale":
		stats.StaleCount++
		stats.ToDeleteCount++
	case "unknown":
		stats.UnknownCount++
		if p.includeUnknown {
			stats.ToDeleteCount++
		}
	case "active":
		stats.ActiveCount++
	case "excluded":
		stats.ExcludedCount++
	}
}
```

- [ ] **Step 2: Commit**
```bash
git add .
git commit -m "feat: add plan generation with categorization and stats"
```

---

## Task 7: Plan Command

**Files:** `internal/cli/plan.go`, `internal/cli/plan_test.go`

- [ ] **Step 1: Implement plan command**
```go
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/vault-secrets-cleanup/internal/correlation"
	"github.com/yourusername/vault-secrets-cleanup/internal/planning"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
	"google.golang.org/protobuf/proto"
)

var (
	planInventory       string
	planAccessData      string
	planOutput          string
	planStalenessDays   int
	planMatchingStrategy string
	planIncludeUnknown  bool
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate deletion plan for stale secrets",
	Long: `Correlate inventory with access data, apply staleness criteria and exclusion rules,
and generate a reviewable deletion plan.`,
	RunE: runPlan,
}

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().StringVar(&planInventory, "inventory", "inventory.pb", "Inventory file from discover")
	planCmd.Flags().StringVar(&planAccessData, "access-data", "access.pb", "Access data from analyze")
	planCmd.Flags().StringVarP(&planOutput, "output", "o", "deletion-plan.pb", "Output file for plan")
	planCmd.Flags().IntVar(&planStalenessDays, "staleness-days", 0, "Override default staleness period")
	planCmd.Flags().StringVar(&planMatchingStrategy, "matching-strategy", "strict", "Matching strategy (strict or path-based)")
	planCmd.Flags().BoolVar(&planIncludeUnknown, "include-unknown", false, "Include secrets with unknown access in plan")
}

func runPlan(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logger.Info("generating deletion plan",
		"inventory", planInventory,
		"access_data", planAccessData,
		"output", planOutput,
	)

	// Load inventory
	inventoryData, err := os.ReadFile(planInventory)
	if err != nil {
		return fmt.Errorf("failed to read inventory: %w", err)
	}

	inventory := &proto.Inventory{}
	if err := proto.Unmarshal(inventoryData, inventory); err != nil {
		return fmt.Errorf("failed to unmarshal inventory: %w", err)
	}

	// Load access data
	accessDataBytes, err := os.ReadFile(planAccessData)
	if err != nil {
		return fmt.Errorf("failed to read access data: %w", err)
	}

	accessData := &proto.AccessData{}
	if err := proto.Unmarshal(accessDataBytes, accessData); err != nil {
		return fmt.Errorf("failed to unmarshal access data: %w", err)
	}

	// Create correlation engine
	corrEngine := correlation.NewEngine(correlation.Config{
		MatchingStrategy: correlation.MatchingStrategy(planMatchingStrategy),
	})

	// Correlate inventory with access data
	accessMap, err := corrEngine.Correlate(inventory, accessData)
	if err != nil {
		return fmt.Errorf("correlation failed: %w", err)
	}

	// Determine staleness days
	stalenessDays := cfg.Staleness.DefaultDays
	if planStalenessDays > 0 {
		stalenessDays = planStalenessDays
	}

	// Create planner
	planner := planning.NewPlanner(planning.Config{
		StalenessConfig: planning.StalenessConfig{
			DefaultDays:      stalenessDays,
			NamespacePolicies: cfg.Staleness.Namespaces,
		},
		ExclusionConfig: planning.ExclusionConfig{
			NamespacePatterns: cfg.Exclusions.Namespaces,
			PathPatterns:      cfg.Exclusions.Paths,
		},
		IncludeUnknown:  planIncludeUnknown,
		MatchingStrategy: correlation.MatchingStrategy(planMatchingStrategy),
	})

	// Generate plan
	planConfig := &proto.PlanConfig{
		StalenessDays:      int32(stalenessDays),
		MatchingStrategy:   planMatchingStrategy,
		IncludeUnknown:     planIncludeUnknown,
		ExclusionPatterns:  append(cfg.Exclusions.Namespaces, cfg.Exclusions.Paths...),
	}

	deletionPlan, err := planner.GeneratePlan(inventory, accessData, accessMap, planConfig)
	if err != nil {
		return fmt.Errorf("plan generation failed: %w", err)
	}

	deletionPlan.InventoryFile = planInventory
	deletionPlan.AccessFile = planAccessData

	// Marshal to protobuf
	planData, err := proto.Marshal(deletionPlan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Write to file
	if err := os.WriteFile(planOutput, planData, 0644); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}

	logger.Info("plan generation complete",
		"total_secrets", deletionPlan.Stats.TotalSecrets,
		"to_delete", deletionPlan.Stats.ToDeleteCount,
		"output", planOutput,
	)

	fmt.Printf("✓ Plan generation complete\n")
	fmt.Printf("  Total secrets: %d\n", deletionPlan.Stats.TotalSecrets)
	fmt.Printf("  Stale: %d\n", deletionPlan.Stats.StaleCount)
	fmt.Printf("  Unknown: %d\n", deletionPlan.Stats.UnknownCount)
	fmt.Printf("  Active: %d\n", deletionPlan.Stats.ActiveCount)
	fmt.Printf("  Excluded: %d\n", deletionPlan.Stats.ExcludedCount)
	fmt.Printf("  To delete: %d\n", deletionPlan.Stats.ToDeleteCount)
	fmt.Printf("  Output: %s\n", planOutput)

	return nil
}
```

- [ ] **Step 2: Test plan command**

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add plan command for deletion plan generation"
```

---

## Task 8: Report Generation

**Files:** `internal/cli/report.go`, `internal/reporting/generator.go`, `internal/reporting/markdown.go`

- [ ] **Step 1: Implement report command** (simplified for brevity)
```go
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourusername/vault-secrets-cleanup/internal/reporting"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
	"google.golang.org/protobuf/proto"
)

var (
	reportPlan   string
	reportFormat string
	reportOutput string
)

var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Generate report from deletion plan",
	RunE:  runReport,
}

func init() {
	rootCmd.AddCommand(reportCmd)

	reportCmd.Flags().StringVar(&reportPlan, "plan", "deletion-plan.pb", "Deletion plan file")
	reportCmd.Flags().StringVar(&reportFormat, "format", "markdown", "Report format (markdown, json, csv)")
	reportCmd.Flags().StringVarP(&reportOutput, "output", "o", "", "Output file (default: stdout)")
}

func runReport(cmd *cobra.Command, args []string) error {
	// Load plan
	planData, err := os.ReadFile(reportPlan)
	if err != nil {
		return fmt.Errorf("failed to read plan: %w", err)
	}

	plan := &proto.DeletionPlan{}
	if err := proto.Unmarshal(planData, plan); err != nil {
		return fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	// Generate report
	generator := reporting.NewGenerator()
	report, err := generator.Generate(plan, reportFormat)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	// Output report
	if reportOutput != "" {
		if err := os.WriteFile(reportOutput, []byte(report), 0644); err != nil {
			return fmt.Errorf("failed to write report: %w", err)
		}
		fmt.Printf("✓ Report written to %s\n", reportOutput)
	} else {
		fmt.Println(report)
	}

	return nil
}
```

- [ ] **Step 2: Implement report generator** (basic markdown format)

- [ ] **Step 3: Test report command**

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "feat: add report command for plan visualization"
```

---

## Phase 4 Completion Checklist

- [ ] Protocol Buffer schema for deletion plans defined
- [ ] Matching strategies (strict and path-based) implemented
- [ ] Correlation engine matches inventory with access data
- [ ] Staleness calculation with per-namespace policies
- [ ] Exclusion rules engine with glob patterns
- [ ] Plan generation categorizes secrets correctly
- [ ] Plan command creates reviewable deletion plans
- [ ] Report command generates human-readable reports
- [ ] All unit tests pass
- [ ] Integration tests pass

---

## Next Steps

After completing Phase 4, proceed to Phase 5: Deletion by loading [`2026-04-10-phase5-deletion.md`](2026-04-10-phase5-deletion.md)