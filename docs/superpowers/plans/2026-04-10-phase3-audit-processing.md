# Phase 3: Audit Processing - Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a high-performance audit log processor that streams and parses large Vault audit logs (10GB+) to extract access patterns for secrets, supporting multiple file formats and compression.

**Architecture:** Streaming JSON parser processes logs in chunks without loading entire files into memory. Concurrent workers process multiple files in parallel. Protocol Buffers store access data efficiently.

**Tech Stack:** Go 1.21+, streaming JSON parser, compression libraries (gzip, bzip2, xz), Protocol Buffers

---

## File Structure

```
vault-secrets-cleanup/
├── pkg/proto/
│   ├── access.proto              # Protobuf schema for access data
│   └── access.pb.go              # Generated code
├── internal/
│   ├── cli/
│   │   ├── analyze.go            # Analyze command
│   │   ├── import.go             # Import command
│   │   └── analyze_test.go
│   └── audit/
│       ├── parser.go             # Streaming JSON parser
│       ├── parser_test.go
│       ├── extractor.go          # Access event extraction
│       ├── extractor_test.go
│       ├── aggregator.go         # Access data aggregation
│       ├── aggregator_test.go
│       └── compression.go        # Compression support
└── tests/integration/
    └── audit_test.go             # Integration tests
```

---

## Task 1: Access Data Protocol Buffer Schema

**Files:** `pkg/proto/access.proto`, `pkg/proto/access.pb.go`

- [ ] **Step 1: Create access.proto**
```protobuf
syntax = "proto3";

package proto;

option go_package = "github.com/yourusername/vault-secrets-cleanup/pkg/proto";

// AccessData represents access patterns for secrets
message AccessData {
  string created_at = 1;           // ISO 8601 timestamp
  string source = 2;               // "audit_log" or "external"
  repeated AccessRecord records = 3;
  AccessStats stats = 4;
}

// AccessRecord represents a single secret's access pattern
message AccessRecord {
  string namespace_path = 1;       // Namespace path
  string namespace_id = 2;         // Namespace ID (optional)
  string mount_path = 3;           // Mount path
  string mount_accessor = 4;       // Mount accessor (optional)
  string secret_path = 5;          // Secret path
  string last_accessed = 6;        // ISO 8601 timestamp
  string access_type = 7;          // "read", "write", "delete"
  int32 access_count = 8;          // Total access count (optional)
}

// AccessStats contains summary statistics
message AccessStats {
  int32 total_records = 1;
  int32 read_count = 2;
  int32 write_count = 3;
  int32 delete_count = 4;
  int32 unique_secrets = 5;
}
```

- [ ] **Step 2: Generate Go code**
```bash
protoc --go_out=. --go_opt=paths=source_relative pkg/proto/access.proto
```

- [ ] **Step 3: Test generation**
```bash
make proto
go build ./pkg/proto/...
```

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "feat: add Protocol Buffer schema for access data"
```

---

## Task 2: Streaming JSON Parser

**Files:** `internal/audit/parser.go`, `internal/audit/parser_test.go`

- [ ] **Step 1: Write parser tests**
```go
package audit

import (
	"bytes"
	"context"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParser_Parse(t *testing.T) {
	t.Run("parses valid audit log", func(t *testing.T) {
		log := `{"time":"2024-01-01T00:00:00Z","type":"response","request":{"path":"secret/data/test"}}
{"time":"2024-01-01T00:01:00Z","type":"response","request":{"path":"secret/data/test2"}}`

		parser := NewParser()
		events := []AuditEvent{}

		err := parser.Parse(context.Background(), bytes.NewReader([]byte(log)), func(event AuditEvent) error {
			events = append(events, event)
			return nil
		})

		require.NoError(t, err)
		assert.Len(t, events, 2)
	})

	t.Run("handles malformed JSON", func(t *testing.T) {
		log := `{"time":"2024-01-01T00:00:00Z","type":"response"
{"time":"2024-01-01T00:01:00Z","type":"response"}`

		parser := NewParser()
		events := []AuditEvent{}

		err := parser.Parse(context.Background(), bytes.NewReader([]byte(log)), func(event AuditEvent) error {
			events = append(events, event)
			return nil
		})

		// Should skip malformed line and continue
		assert.NoError(t, err)
		assert.Len(t, events, 1)
	})
}
```

- [ ] **Step 2: Run tests to verify failure**
```bash
go test ./internal/audit/... -v
```

- [ ] **Step 3: Implement AuditEvent struct**
```go
package audit

import (
	"time"
)

type AuditEvent struct {
	Time      time.Time
	Type      string
	Request   RequestInfo
	Auth      AuthInfo
	Response  ResponseInfo
}

type RequestInfo struct {
	ID            string
	Operation     string
	Path          string
	Namespace     NamespaceInfo
	MountAccessor string
	MountType     string
}

type NamespaceInfo struct {
	ID   string
	Path string
}

type AuthInfo struct {
	TokenType string
	Policies  []string
}

type ResponseInfo struct {
	MountType string
}
```

- [ ] **Step 4: Implement streaming parser**
```go
package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
)

type Parser struct {
	bufferSize int
}

type EventHandler func(AuditEvent) error

func NewParser() *Parser {
	return &Parser{
		bufferSize: 64 * 1024, // 64KB buffer
	}
}

func (p *Parser) Parse(ctx context.Context, reader io.Reader, handler EventHandler) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, p.bufferSize), p.bufferSize)

	lineNum := 0
	for scanner.Scan() {
		lineNum++

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse JSON line
		var rawEvent map[string]interface{}
		if err := json.Unmarshal(line, &rawEvent); err != nil {
			// Skip malformed lines but log warning
			continue
		}

		// Convert to AuditEvent
		event, err := p.parseEvent(rawEvent)
		if err != nil {
			// Skip events we can't parse
			continue
		}

		// Call handler
		if err := handler(event); err != nil {
			return fmt.Errorf("handler error at line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %w", err)
	}

	return nil
}

func (p *Parser) parseEvent(raw map[string]interface{}) (AuditEvent, error) {
	event := AuditEvent{}

	// Parse time
	if timeStr, ok := raw["time"].(string); ok {
		t, err := time.Parse(time.RFC3339, timeStr)
		if err == nil {
			event.Time = t
		}
	}

	// Parse type
	if eventType, ok := raw["type"].(string); ok {
		event.Type = eventType
	}

	// Parse request
	if request, ok := raw["request"].(map[string]interface{}); ok {
		event.Request = p.parseRequest(request)
	}

	// Parse auth
	if auth, ok := raw["auth"].(map[string]interface{}); ok {
		event.Auth = p.parseAuth(auth)
	}

	return event, nil
}

func (p *Parser) parseRequest(raw map[string]interface{}) RequestInfo {
	req := RequestInfo{}

	if id, ok := raw["id"].(string); ok {
		req.ID = id
	}
	if op, ok := raw["operation"].(string); ok {
		req.Operation = op
	}
	if path, ok := raw["path"].(string); ok {
		req.Path = path
	}
	if accessor, ok := raw["mount_accessor"].(string); ok {
		req.MountAccessor = accessor
	}
	if mountType, ok := raw["mount_type"].(string); ok {
		req.MountType = mountType
	}

	// Parse namespace
	if ns, ok := raw["namespace"].(map[string]interface{}); ok {
		if id, ok := ns["id"].(string); ok {
			req.Namespace.ID = id
		}
		if path, ok := ns["path"].(string); ok {
			req.Namespace.Path = path
		}
	}

	return req
}

func (p *Parser) parseAuth(raw map[string]interface{}) AuthInfo {
	auth := AuthInfo{}

	if tokenType, ok := raw["token_type"].(string); ok {
		auth.TokenType = tokenType
	}

	if policies, ok := raw["policies"].([]interface{}); ok {
		for _, p := range policies {
			if policy, ok := p.(string); ok {
				auth.Policies = append(auth.Policies, policy)
			}
		}
	}

	return auth
}
```

- [ ] **Step 5: Run tests to verify pass**
```bash
go test ./internal/audit/... -v
```

- [ ] **Step 6: Commit**
```bash
git add .
git commit -m "feat: add streaming JSON parser for audit logs"
```

---

## Task 3: Access Event Extraction

**Files:** `internal/audit/extractor.go`, `internal/audit/extractor_test.go`

- [ ] **Step 1: Write extractor tests**
```go
package audit

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
)

func TestExtractor_Extract(t *testing.T) {
	t.Run("extracts KV v2 read", func(t *testing.T) {
		event := AuditEvent{
			Time: time.Now(),
			Type: "response",
			Request: RequestInfo{
				Operation:     "read",
				Path:          "secret/data/myapp/config",
				MountAccessor: "kv_12345",
				MountType:     "kv",
				Namespace: NamespaceInfo{
					ID:   "ns123",
					Path: "prod",
				},
			},
		}

		extractor := NewExtractor()
		record := extractor.Extract(event)

		assert.NotNil(t, record)
		assert.Equal(t, "prod", record.NamespacePath)
		assert.Equal(t, "secret/", record.MountPath)
		assert.Equal(t, "myapp/config", record.SecretPath)
		assert.Equal(t, "read", record.AccessType)
	})

	t.Run("skips non-KV events", func(t *testing.T) {
		event := AuditEvent{
			Request: RequestInfo{
				MountType: "pki",
			},
		}

		extractor := NewExtractor()
		record := extractor.Extract(event)

		assert.Nil(t, record)
	})
}
```

- [ ] **Step 2: Implement extractor**
```go
package audit

import (
	"strings"
	"time"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type Extractor struct{}

func NewExtractor() *Extractor {
	return &Extractor{}
}

func (e *Extractor) Extract(event AuditEvent) *proto.AccessRecord {
	// Only process response events
	if event.Type != "response" {
		return nil
	}

	// Only process KV mounts
	if !isKVMount(event.Request.MountType) {
		return nil
	}

	// Extract secret path from request path
	secretPath, mountPath, ok := e.extractSecretPath(event.Request.Path)
	if !ok {
		return nil
	}

	// Determine access type
	accessType := e.determineAccessType(event.Request.Operation)
	if accessType == "" {
		return nil
	}

	return &proto.AccessRecord{
		NamespacePath: event.Request.Namespace.Path,
		NamespaceId:   event.Request.Namespace.ID,
		MountPath:     mountPath,
		MountAccessor: event.Request.MountAccessor,
		SecretPath:    secretPath,
		LastAccessed:  event.Time.UTC().Format(time.RFC3339),
		AccessType:    accessType,
		AccessCount:   1,
	}
}

func (e *Extractor) extractSecretPath(path string) (secretPath, mountPath string, ok bool) {
	// KV v2 paths: <mount>/data/<secret> or <mount>/metadata/<secret>
	// KV v1 paths: <mount>/<secret>

	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", false
	}

	// Check for KV v2 patterns
	if len(parts) >= 3 && (parts[1] == "data" || parts[1] == "metadata") {
		mountPath = parts[0] + "/"
		secretPath = strings.Join(parts[2:], "/")
		return secretPath, mountPath, true
	}

	// KV v1 pattern
	mountPath = parts[0] + "/"
	secretPath = strings.Join(parts[1:], "/")
	return secretPath, mountPath, true
}

func (e *Extractor) determineAccessType(operation string) string {
	switch operation {
	case "read":
		return "read"
	case "create", "update":
		return "write"
	case "delete":
		return "delete"
	default:
		return ""
	}
}

func isKVMount(mountType string) bool {
	return mountType == "kv" || mountType == "generic"
}
```

- [ ] **Step 3: Run tests to verify pass**

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "feat: add access event extraction from audit logs"
```

---

## Task 4: Access Data Aggregation

**Files:** `internal/audit/aggregator.go`, `internal/audit/aggregator_test.go`

- [ ] **Step 1: Write aggregator tests**

- [ ] **Step 2: Implement Aggregator**
```go
package audit

import (
	"sync"
	"time"

	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
)

type Aggregator struct {
	mu      sync.Mutex
	records map[string]*proto.AccessRecord
}

func NewAggregator() *Aggregator {
	return &Aggregator{
		records: make(map[string]*proto.AccessRecord),
	}
}

func (a *Aggregator) Add(record *proto.AccessRecord) {
	if record == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	key := a.makeKey(record)

	existing, exists := a.records[key]
	if !exists {
		a.records[key] = record
		return
	}

	// Update with latest access time
	existingTime, _ := time.Parse(time.RFC3339, existing.LastAccessed)
	newTime, _ := time.Parse(time.RFC3339, record.LastAccessed)

	if newTime.After(existingTime) {
		existing.LastAccessed = record.LastAccessed
		existing.AccessType = record.AccessType
	}

	existing.AccessCount += record.AccessCount
}

func (a *Aggregator) makeKey(record *proto.AccessRecord) string {
	return record.NamespaceId + "|" + record.MountAccessor + "|" + record.SecretPath
}

func (a *Aggregator) GetRecords() []*proto.AccessRecord {
	a.mu.Lock()
	defer a.mu.Unlock()

	records := make([]*proto.AccessRecord, 0, len(a.records))
	for _, record := range a.records {
		records = append(records, record)
	}

	return records
}

func (a *Aggregator) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.records)
}
```

- [ ] **Step 3: Run tests to verify pass**

- [ ] **Step 4: Commit**
```bash
git add .
git commit -m "feat: add access data aggregation with deduplication"
```

---

## Task 5: Compression Support

**Files:** `internal/audit/compression.go`

- [ ] **Step 1: Implement compression reader**
```go
package audit

import (
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ulikunitz/xz"
)

func OpenFile(path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// Detect compression by file extension
	if strings.HasSuffix(path, ".gz") {
		reader, err := gzip.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return &compressedReader{file: file, reader: reader}, nil
	}

	if strings.HasSuffix(path, ".bz2") {
		reader := bzip2.NewReader(file)
		return &compressedReader{file: file, reader: reader}, nil
	}

	if strings.HasSuffix(path, ".xz") {
		reader, err := xz.NewReader(file)
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create xz reader: %w", err)
		}
		return &compressedReader{file: file, reader: reader}, nil
	}

	// No compression
	return file, nil
}

type compressedReader struct {
	file   *os.File
	reader io.Reader
}

func (r *compressedReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

func (r *compressedReader) Close() error {
	// Close the underlying file
	return r.file.Close()
}
```

- [ ] **Step 2: Add xz dependency**
```bash
go get github.com/ulikunitz/xz@latest
go mod tidy
```

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add compression support for gzip, bzip2, and xz"
```

---

## Task 6: Analyze Command

**Files:** `internal/cli/analyze.go`, `internal/cli/analyze_test.go`

- [ ] **Step 1: Implement analyze command**
```go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/vault-secrets-cleanup/internal/audit"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
	"google.golang.org/protobuf/proto"
)

var (
	analyzeAuditLog  string
	analyzeInventory string
	analyzeOutput    string
)

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze audit logs to extract access patterns",
	Long: `Parse Vault audit logs to determine when secrets were last accessed.
Supports JSON and JSONL formats, with gzip, bzip2, and xz compression.`,
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	analyzeCmd.Flags().StringVar(&analyzeAuditLog, "audit-log", "", "Audit log file or glob pattern (required)")
	analyzeCmd.Flags().StringVar(&analyzeInventory, "inventory", "", "Inventory file from discover command")
	analyzeCmd.Flags().StringVarP(&analyzeOutput, "output", "o", "access.pb", "Output file for access data")

	analyzeCmd.MarkFlagRequired("audit-log")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	logger.Info("starting audit log analysis",
		"audit_log", analyzeAuditLog,
		"output", analyzeOutput,
	)

	// Find audit log files
	files, err := filepath.Glob(analyzeAuditLog)
	if err != nil {
		return fmt.Errorf("failed to glob audit logs: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no audit log files found matching: %s", analyzeAuditLog)
	}

	logger.Info("found audit log files", "count", len(files))

	// Create parser and aggregator
	parser := audit.NewParser()
	extractor := audit.NewExtractor()
	aggregator := audit.NewAggregator()

	// Process each file
	for _, file := range files {
		logger.Info("processing file", "file", file)

		if err := processAuditFile(ctx, file, parser, extractor, aggregator); err != nil {
			logger.Error("failed to process file", "file", file, "error", err)
			continue
		}
	}

	// Create access data
	accessData := &proto.AccessData{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Source:    "audit_log",
		Records:   aggregator.GetRecords(),
		Stats:     calculateAccessStats(aggregator.GetRecords()),
	}

	// Marshal to protobuf
	data, err := proto.Marshal(accessData)
	if err != nil {
		return fmt.Errorf("failed to marshal access data: %w", err)
	}

	// Write to file
	if err := os.WriteFile(analyzeOutput, data, 0644); err != nil {
		return fmt.Errorf("failed to write access data: %w", err)
	}

	logger.Info("analysis complete",
		"records", accessData.Stats.TotalRecords,
		"unique_secrets", accessData.Stats.UniqueSecrets,
		"output", analyzeOutput,
	)

	fmt.Printf("✓ Analysis complete\n")
	fmt.Printf("  Total records: %d\n", accessData.Stats.TotalRecords)
	fmt.Printf("  Unique secrets: %d\n", accessData.Stats.UniqueSecrets)
	fmt.Printf("  Output: %s\n", analyzeOutput)

	return nil
}

func processAuditFile(ctx context.Context, path string, parser *audit.Parser, extractor *audit.Extractor, aggregator *audit.Aggregator) error {
	reader, err := audit.OpenFile(path)
	if err != nil {
		return err
	}
	defer reader.Close()

	return parser.Parse(ctx, reader, func(event audit.AuditEvent) error {
		record := extractor.Extract(event)
		aggregator.Add(record)
		return nil
	})
}

func calculateAccessStats(records []*proto.AccessRecord) *proto.AccessStats {
	stats := &proto.AccessStats{
		TotalRecords:  int32(len(records)),
		UniqueSecrets: int32(len(records)),
	}

	for _, record := range records {
		switch record.AccessType {
		case "read":
			stats.ReadCount++
		case "write":
			stats.WriteCount++
		case "delete":
			stats.DeleteCount++
		}
	}

	return stats
}
```

- [ ] **Step 2: Test analyze command**
```bash
make build
./bin/vault-secrets-cleanup analyze --audit-log "/var/log/vault/audit-*.log" --output access.pb
```

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add analyze command for audit log processing"
```

---

## Task 7: Import Command

**Files:** `internal/cli/import.go`

- [ ] **Step 1: Implement import command**
```go
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/yourusername/vault-secrets-cleanup/pkg/proto"
	"google.golang.org/protobuf/proto"
)

var (
	importInput  string
	importOutput string
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import access data from external sources",
	Long: `Import access data from external log management systems (Splunk, Elasticsearch, etc.).
Accepts JSON file with array of access records.`,
	RunE: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVarP(&importInput, "input", "i", "", "Input JSON file (required)")
	importCmd.Flags().StringVarP(&importOutput, "output", "o", "access.pb", "Output file for access data")

	importCmd.MarkFlagRequired("input")
}

func runImport(cmd *cobra.Command, args []string) error {
	logger.Info("importing access data", "input", importInput)

	// Read JSON file
	data, err := os.ReadFile(importInput)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Parse JSON
	var records []*proto.AccessRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Create access data
	accessData := &proto.AccessData{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Source:    "external",
		Records:   records,
		Stats:     calculateAccessStats(records),
	}

	// Marshal to protobuf
	pbData, err := proto.Marshal(accessData)
	if err != nil {
		return fmt.Errorf("failed to marshal access data: %w", err)
	}

	// Write to file
	if err := os.WriteFile(importOutput, pbData, 0644); err != nil {
		return fmt.Errorf("failed to write access data: %w", err)
	}

	logger.Info("import complete",
		"records", len(records),
		"output", importOutput,
	)

	fmt.Printf("✓ Import complete\n")
	fmt.Printf("  Records: %d\n", len(records))
	fmt.Printf("  Output: %s\n", importOutput)

	return nil
}
```

- [ ] **Step 2: Test import command**

- [ ] **Step 3: Commit**
```bash
git add .
git commit -m "feat: add import command for external access data"
```

---

## Phase 3 Completion Checklist

- [ ] Protocol Buffer schema for access data defined
- [ ] Streaming JSON parser handles large files efficiently
- [ ] Access event extraction works for KV v1 and v2
- [ ] Aggregation deduplicates and tracks latest access
- [ ] Compression support for gzip, bzip2, xz
- [ ] Analyze command processes 10GB+ logs in <30 minutes
- [ ] Import command accepts external access data
- [ ] Memory usage <500MB during processing
- [ ] All unit tests pass
- [ ] Integration tests pass

---

## Next Steps

After completing Phase 3, proceed to Phase 4: Correlation & Planning by loading [`2026-04-10-phase4-correlation-planning.md`](2026-04-10-phase4-correlation-planning.md)