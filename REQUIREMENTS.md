# Vault Secrets Cleanup Tool - Requirements Document

## Executive Summary

A Go-based CLI tool for HashiCorp Vault Enterprise that helps DevOps and Platform teams identify and remove stale static secrets from KV engines based on access patterns derived from audit logs.

**Target Scale**: Large deployments (1-10GB+ audit logs, tens of millions of entries)
**Primary Users**: Operators running from workstations
**Vault Support**: Enterprise (with namespace support required)

---

## 1. Functional Requirements

### 1.1 Secrets Discovery & Inventory

**FR-1.1.1**: Discover all namespaces in Vault cluster
- Query Vault API to enumerate all namespaces recursively
- Support nested namespace hierarchies
- Handle permission boundaries gracefully (skip inaccessible namespaces with warning)

**FR-1.1.2**: Discover KV mounts per namespace
- Identify all KV v1 and KV v2 mounts in each namespace
- Extract mount path, mount accessor, and KV version
- Filter for KV engines only (exclude other secret engines)

**FR-1.1.3**: Enumerate secrets paths
- Recursively list all secret paths within each KV mount
- Support both KV v1 and KV v2 path structures
- Handle large secret trees efficiently (streaming/pagination)
- Respect Vault's list operation limits

**FR-1.1.4**: Export secrets inventory
- Generate structured inventory file with:
  - Namespace path
  - Mount path
  - Mount accessor
  - Secret path (full path)
  - KV version (v1 or v2)
  - Discovery timestamp

### 1.2 Audit Log Processing

**FR-1.2.1**: Parse Vault audit logs
- Support JSON and JSONL audit log formats
- Handle compressed logs (gzip, bzip2, xz)
- Process logs in streaming fashion for large files (1GB-10GB+)
- Support multiple audit log files (merge processing)

**FR-1.2.2**: Extract access events
- Identify read operations on KV secrets
- Identify write/update operations on KV secrets
- Identify delete operations on KV secrets
- Extract timestamp, namespace path, mount path, mount accessor, and secret path
- Handle both KV v1 and KV v2 API paths

**FR-1.2.3**: Determine last access timestamps
- Track most recent access (read or write) per secret
- Track type of most recent access (read/write/delete) per secret
- Handle secrets with multiple access events efficiently
- Support configurable access event types (read-only, write-only, or both)

**FR-1.2.4**: Export access data
- Generate structured access file with:
  - Namespace path
  - Mount path
  - Mount accessor
  - Secret path
  - Last accessed timestamp
  - Access type (read/write/delete)
  - Access count (optional)
- Support incremental updates (merge with existing data)

**FR-1.2.5**: Import external access data
- Support importing access data from external sources (e.g., log management solutions)
- Accept JSON file with array of access records containing:
  - `namespace_path` (required)
  - `mount_path` (required)
  - `mount_accessor` (optional)
  - `secret_path` (required)
  - `timestamp` (required, ISO 8601 or Unix timestamp)
  - `access_type` (optional, defaults to "read")
- Convert imported data to internal access data format
- Merge with existing access data if present
- Validate required fields and data types
- Support both single-file and multi-file imports

### 1.3 Stale Secrets Identification

**FR-1.3.1**: Correlate inventory and access data
- Match secrets from inventory with access records
- Support configurable matching strategies:
  - **Strict matching** (default): Match by namespace path + mount accessor + secret path
  - **Path-based matching**: Match by namespace path + mount path + secret path (ignores accessors)
- User-configurable via CLI flag or configuration file
- Log matching strategy used and any fallback occurrences
- Handle cases where mount accessor is missing in access data

**FR-1.3.2**: Apply staleness criteria
- User-configurable staleness period (e.g., 365 days)
- Identify secrets not accessed within the period
- Identify secrets with unknown last access (never in audit logs)
- Support different staleness periods per namespace (optional)

**FR-1.3.3**: Categorize secrets
- **Stale**: Not accessed within configured period
- **Unknown**: No access records found in audit logs
- **Active**: Accessed within configured period
- **Excluded**: Matching exclusion patterns

**FR-1.3.4**: Generate stale secrets report
- List all stale secrets with metadata
- List all unknown-access secrets separately
- Include statistics: total secrets, stale count, unknown count, active count
- Support filtering and sorting options

### 1.4 Deletion Planning

**FR-1.4.1**: Create deletion plan
- Generate plan file listing secrets to delete
- Include metadata: namespace, mount, path, last accessed, reason
- Support dry-run mode (plan only, no deletion)
- Allow user review and approval workflow

**FR-1.4.2**: Handle unknown-access secrets
- Separate flag to include/exclude unknown-access secrets in plan
- Default: exclude unknown-access secrets (opt-in required)
- Clear warnings when including unknown-access secrets

**FR-1.4.3**: Apply exclusion rules
- Support path-based exclusion patterns (glob/regex)
- Support namespace-based exclusions
- Support mount-based exclusions
- Load exclusions from configuration file

**FR-1.4.4**: Plan file format
- Track deletion status per secret (pending/deleted/failed)
- Include deletion timestamp when executed
- Support plan versioning and history
- Enable plan resumption after interruption

### 1.5 Secrets Deletion

**FR-1.5.1**: Execute deletion plan
- Read approved deletion plan file
- Display summary before execution:
  - Total secrets to delete
  - Number of affected namespaces
  - Number of affected KV mounts
  - Estimated time to completion
- Require explicit user confirmation

**FR-1.5.2**: Perform deletions
- Delete secrets according to plan
- Handle KV v1 and KV v2 deletion semantics
- Perform hard delete for both KV v1 and KV v2
- Update plan file with deletion status in real-time

**FR-1.5.3**: Deletion safety features
- Implement circuit breaker (stop after N consecutive failures)
- Support deletion limits (max secrets per run)
- Require explicit confirmation before deletion

**FR-1.5.4**: Track deletion progress
- Update plan file with deletion timestamps
- Mark secrets as deleted/failed with error details
- Generate deletion summary report
- Support resumable deletions (skip already-deleted secrets)

---

## 2. Non-Functional Requirements

### 2.1 Performance

**NFR-2.1.1**: Rate limiting
- Configurable requests per second to Vault API
- Default: 100 requests/second (conservative)
- Per-operation rate limits (discovery, deletion)
- Adaptive rate limiting based on Vault response times

**NFR-2.1.2**: Retry logic
- Automatic retry for 5XX server errors
- Exponential backoff with jitter
- Configurable max retry attempts (default: 3)
- Configurable retry delay (default: 1s, 2s, 4s)

**NFR-2.1.3**: Concurrency
- Parallel processing of namespaces/mounts where safe
- Configurable worker pool size
- Respect rate limits across concurrent operations
- Graceful handling of context cancellation

**NFR-2.1.4**: Memory efficiency
- Stream large audit logs (no full load into memory)
- Process secrets inventory in chunks
- Configurable buffer sizes for streaming operations
- Memory usage target: <500MB for 10GB audit log processing

**NFR-2.1.5**: Audit log processing performance
- Target: Process 10GB audit log in <30 minutes
- Support parallel processing of multiple log files
- Efficient JSON parsing (use streaming parser)
- Progress tracking with ETA

### 2.2 Reliability

**NFR-2.2.1**: Error handling
- Graceful handling of network failures
- Proper error messages with context
- Continue processing on non-fatal errors
- Detailed error logging for troubleshooting

**NFR-2.2.2**: Data integrity
- Atomic file writes (write to temp, then rename)
- Validate data format before processing
- Checksum verification for data files
- Prevent partial/corrupted output files

**NFR-2.2.3**: Idempotency
- Safe to re-run operations multiple times
- Skip already-processed items
- Merge results from multiple runs
- No duplicate deletions

**NFR-2.2.4**: Observability
- Progress bars for all long-running operations
- Structured logging (JSON format option)

### 2.3 Security

**NFR-2.3.1**: Authentication
- Require VAULT_TOKEN environment variable
- Token renewal for long-running operations

**NFR-2.3.2**: Authorization
- Require minimum Vault policies:
  - List namespaces
  - List mounts per namespace
  - List secrets per mount
  - Read secret metadata (for KV v2)
  - Delete secrets (for deletion operations)
- Validate permissions before operations
- Clear error messages for permission issues

**NFR-2.3.3**: Data protection
- Encrypt sensitive data files at rest (optional)
- Secure file permissions (0600 for sensitive files)
- Support secure deletion of temporary files
- No caching of secret values

### 2.4 Usability

**NFR-2.4.1**: CLI interface
- Intuitive command structure
- Comprehensive help text
- Examples in help output
- Shell completion support (bash, zsh, fish)

**NFR-2.4.2**: Configuration
- Support configuration file (YAML)
- Environment variable overrides
- CLI flag overrides (highest priority)
- Sensible defaults for all options

**NFR-2.4.3**: Output formats
- Human-readable output (default)
- JSON output for automation
- CSV export for spreadsheet analysis
- Markdown reports for documentation

**NFR-2.4.4**: User feedback
- Clear progress indicators
- Informative error messages
- Warnings for potentially dangerous operations
- Success confirmations with summaries

### 2.5 Maintainability

**NFR-2.5.1**: Code quality
- Go best practices and idioms
- Comprehensive unit tests (>80% coverage)
- Integration tests with Vault
- Linting and static analysis (golangci-lint)

**NFR-2.5.2**: Documentation
- README with quick start guide
- Detailed usage documentation
- Architecture documentation
- API documentation (godoc)

**NFR-2.5.3**: Versioning
- Semantic versioning (semver)
- Changelog maintenance
- Backward compatibility guarantees
- Migration guides for breaking changes

**NFR-2.5.4**: Extensibility
- Plugin architecture for custom filters (future)
- Configurable output formats
- Extensible auth method support
- Modular design for easy feature additions

---

## 3. Data Format Recommendations

### 3.1 Recommended Format: Protocol Buffers (protobuf)

**Advantages over JSON**:
- **Size**: 3-10x smaller than JSON (binary format)
- **Speed**: 5-10x faster parsing/serialization
- **Schema**: Built-in schema validation and evolution
- **Streaming**: Excellent streaming support for large datasets
- **Type safety**: Strong typing prevents errors

**Use cases**:
- Secrets inventory files
- Access data files
- Deletion plan files
- Internal data exchange

**Implementation**:
```protobuf
message SecretInventory {
  repeated SecretEntry secrets = 1;
  string generated_at = 2;
  string vault_address = 3;
}

message SecretEntry {
  string namespace_path = 1;
  string mount_path = 2;
  string mount_accessor = 3;
  string secret_path = 4;
  string kv_version = 5;
  int64 discovered_at = 6;
}
```

### 3.2 Alternative Format: Apache Parquet

**Advantages**:
- **Columnar storage**: Excellent for analytics queries
- **Compression**: Very high compression ratios
- **Query performance**: Fast filtering and aggregation
- **Ecosystem**: Wide tool support (Spark, Pandas, DuckDB)

**Use cases**:
- Large-scale audit log processing
- Historical access pattern analysis
- Data warehouse integration

**Trade-offs**:
- More complex than protobuf
- Requires additional dependencies
- Overkill for small datasets

### 3.3 Fallback Format: JSON Lines (JSONL)

**Advantages**:
- **Streaming**: Process line-by-line
- **Human-readable**: Easy debugging
- **Universal**: Works everywhere
- **Append-friendly**: Easy incremental updates

**Use cases**:
- Human-readable reports
- Debugging and development
- Integration with existing JSON tools
- Small to medium datasets

**Recommendation**: Use JSONL for human-facing outputs, protobuf for internal data files.

### 3.4 Format Selection Strategy

| Data Type | Primary Format | Alternative |
|-----------|----------------|-------------|
| Inventory | Protobuf | JSONL |
| Access Data | Protobuf | Parquet |
| Deletion Plan | Protobuf | JSONL |
| Reports | JSONL/Markdown | CSV |
| Configuration | YAML | TOML |
| Logs | JSON | Text |

---

## 4. Additional Requirements

### 4.1 Audit Log Analysis Enhancements

**FR-4.1.1**: Audit log filtering
- Filter by time range
- Filter by namespace
- Filter by operation type
- Pre-filter before processing (performance optimization)

**FR-4.1.2**: Access pattern analysis
- Generate access frequency reports
- Identify access trends over time
- Detect anomalous access patterns
- Export analytics data for visualization

### 4.2 Advanced Deletion Features

**FR-4.2.1**: Deletion strategies
- Immediate deletion
- Gradual deletion (rate-limited over time)

### 4.3 Operational Features

**FR-4.3.1**: Incremental operations
- Incremental inventory updates
- Incremental audit log processing
- Resume interrupted operations
- State persistence between runs

**FR-4.3.2**: Validation and testing
- Validate Vault connectivity
- Validate permissions
- Validate configuration
- Test mode (no actual changes)

**FR-4.3.3**: Performance tuning
- Auto-tune rate limits based on Vault performance
- Adaptive batch sizes
- Connection pooling
- Request caching (where safe)

---

## 5. System Architecture

### 5.1 High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     CLI Interface                            │
│  (cobra/viper for commands, flags, config)                  │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────┴────────────────────────────────────────┐
│                  Core Application Layer                      │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  Discovery   │  │  Audit Log   │  │  Deletion    │     │
│  │  Engine      │  │  Processor   │  │  Engine      │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────┴────────────────────────────────────────┐
│                  Service Layer                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  Vault       │  │  Storage     │  │  Rate        │     │
│  │  Client      │  │  Service     │  │  Limiter     │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└────────────────────┬────────────────────────────────────────┘
                     │
┌────────────────────┴────────────────────────────────────────┐
│                  Infrastructure Layer                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  Vault API   │  │  File System │  │  Logging     │     │
│  │  (HTTP)      │  │  (Protobuf)  │  │  (Zap)       │     │
│  └──────────────┘  └──────────────┘  └──────────────┘     │
└─────────────────────────────────────────────────────────────┘
```

### 5.2 Data Flow Diagram

```
Vault Cluster
     │
     │ API Calls
     ▼
Discovery Engine ────▶ inventory.pb
                            │
Audit Logs                  │
     │                      │
     │ Stream               │
     ▼                      │
Audit Processor ─────▶ access.pb
                            │
                   ┌────────▼─────────┐
                   │ Correlation      │
                   │ Engine           │
                   └────────┬─────────┘
                            │
                   ┌────────▼─────────┐
                   │ deletion-plan.pb │
                   └────────┬─────────┘
                            │
                   ┌────────▼─────────┐
                   │ Deletion Engine  │
                   └────────┬─────────┘
                            │
                   ┌────────▼─────────┐
                   │ Vault Cluster    │
                   │ (Deletions)      │
                   └──────────────────┘
```

### 5.3 Component Descriptions

**Discovery Engine**:
- Enumerates namespaces, mounts, and secrets
- Implements pagination and rate limiting
- Produces secrets inventory

**Audit Processor**:
- Streams audit logs
- Extracts access events
- Builds access timestamp index
- Handles multiple log formats

**Correlation Engine**:
- Matches inventory with access data
- Applies staleness criteria
- Generates deletion candidates
- Applies exclusion rules

**Deletion Engine**:
- Executes deletion plans
- Tracks progress and status
- Implements safety features
- Generates reports

**Vault Client**:
- Abstracts Vault API interactions
- Handles authentication and token renewal
- Implements retry logic
- Manages connection pooling

**Storage Service**:
- Manages data file I/O
- Handles protobuf serialization
- Implements atomic writes
- Provides data validation

**Rate Limiter**:
- Token bucket algorithm
- Per-operation limits
- Adaptive rate adjustment
- Distributed rate limiting (future)

---

## 6. CLI Interface Design

### 6.1 Command Structure

```
vault-secrets-cleanup
├── discover          # Discover secrets and create inventory
├── analyze           # Analyze audit logs for access patterns
├── import            # Import access data from external sources
├── plan              # Create deletion plan
├── execute           # Execute deletion plan
├── report            # Generate reports
├── validate          # Validate configuration and permissions
└── version           # Show version information
```

### 6.2 Command Examples

#### discover
```bash
vault-secrets-cleanup discover \
  --vault-addr https://vault.example.com \
  --output inventory.pb \
  --rate-limit 50 \
  --exclude-namespaces "test/*,dev/*"
```

#### analyze
```bash
vault-secrets-cleanup analyze \
  --audit-log "/var/log/vault/audit-*.log" \
  --inventory inventory.pb \
  --output access.pb \
  --time-range "2023-01-01,2024-01-01"
```

#### import
```bash
vault-secrets-cleanup import \
  --input external-access-data.json \
  --output access.pb \
  --merge  # Optional: merge with existing access data
```

#### plan
```bash
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-period 365d \
  --matching-strategy strict \
  --exclude-patterns exclusions.txt \
  --output deletion-plan.pb
```

#### execute
```bash
vault-secrets-cleanup execute \
  --plan deletion-plan.pb \
  --vault-addr https://vault.example.com \
  --rate-limit 5 \
  --yes
```

#### report
```bash
vault-secrets-cleanup report \
  --inventory inventory.pb \
  --access-data access.pb \
  --type summary \
  --format markdown \
  --output report.md
```

#### validate
```bash
vault-secrets-cleanup validate \
  --vault-addr https://vault.example.com \
  --check-permissions \
  --check-connectivity
```

---

## 7. Configuration File Format

**Format**: YAML

**Location**: `~/.vault-secrets-cleanup.yaml` or via `--config`

**Example**:
```yaml
# Vault connection
vault:
  address: https://vault.example.com
  auth_method: token
  token: ${VAULT_TOKEN}
  namespace: root
  
  tls:
    ca_cert: /path/to/ca.crt
    skip_verify: false

# Rate limiting
rate_limit:
  discovery: 100
  deletion: 10
  audit_processing: 1000

# Concurrency
concurrency:
  discovery_workers: 10
  deletion_workers: 5
  audit_workers: 4

# Retry configuration
retry:
  max_attempts: 3
  initial_delay: 1s
  max_delay: 30s
  multiplier: 2.0

# Correlation configuration
correlation:
  # Matching strategy: strict, path-based, or hybrid
  # - strict: Match by namespace path + mount accessor + secret path (default)
  # - path-based: Match by namespace path + mount path + secret path
  matching_strategy: strict

# Staleness configuration
staleness:
  default_period: 365d
  
  namespaces:
    prod/*: 730
    dev/*: 90
    staging/*: 180

# Exclusions
exclusions:
  namespaces:
    - admin/*
    - system/*
  
  mounts:
    - "*/terraform-state"
    - "*/ci-secrets"
  
  paths:
    - "*/bootstrap/*"
    - "*/root-token"

# Deletion options
deletion:
  max_deletions_per_run: 1000
  circuit_breaker:
    failure_threshold: 10
    timeout: 5m

# Output options
output:
  format: protobuf
  compression: gzip
  
# Logging
logging:
  level: info
  format: json
  file: /var/log/vault-secrets-cleanup.log
```

---

## 8. Implementation Roadmap

### Phase 1: Foundation (Weeks 1-2)
- [ ] Project setup and structure
- [ ] CLI framework (cobra/viper)
- [ ] Configuration management
- [ ] Vault client wrapper
- [ ] Rate limiter implementation
- [ ] Retry logic implementation
- [ ] Logging infrastructure
- [ ] Basic unit tests

### Phase 2: Discovery (Weeks 3-4)
- [ ] Namespace enumeration
- [ ] Mount discovery
- [ ] Secret path enumeration
- [ ] Inventory data model (protobuf)
- [ ] Inventory export
- [ ] Progress tracking
- [ ] Integration tests
- [ ] Documentation

### Phase 3: Audit Processing (Weeks 5-7)
- [ ] Audit log parser (streaming)
- [ ] Access event extraction
- [ ] Access data model (protobuf)
- [ ] Multi-file processing
- [ ] Compression support
- [ ] Performance optimization
- [ ] Large-scale testing
- [ ] Documentation

### Phase 4: Correlation & Planning (Weeks 8-9)
- [ ] Inventory-access correlation
- [ ] Staleness calculation
- [ ] Exclusion rules engine
- [ ] Deletion plan model (protobuf)
- [ ] Plan generation
- [ ] Report generation
- [ ] Unit tests
- [ ] Documentation

### Phase 5: Deletion (Weeks 10-11)
- [ ] Deletion engine
- [ ] Plan execution
- [ ] Progress tracking
- [ ] Backup/restore
- [ ] Circuit breaker
- [ ] Status updates
- [ ] Integration tests
- [ ] Documentation

### Phase 6: Polish & Release (Weeks 12-13)
- [ ] End-to-end testing
- [ ] Performance optimization
- [ ] Security audit
- [ ] Documentation review
- [ ] Example configurations
- [ ] Release preparation
- [ ] CI/CD pipeline
- [ ] Initial release (v1.0.0)

### Phase 7: Enhancements (Post-v1.0)
- [ ] Multi-cluster support
- [ ] Advanced reporting
- [ ] Notification system
- [ ] GitOps integration
- [ ] Metrics export
- [ ] Dashboard generation
- [ ] Plugin system
- [ ] Cloud storage support

---

## 9. Success Criteria

### 9.1 Functional Success
- Successfully discover 100k+ secrets across multiple namespaces
- Process 10GB audit log in <30 minutes
- Delete 10k secrets with <1% error rate
- Support all major Vault auth methods
- Generate accurate reports and plans

### 9.2 Performance Success
- Memory usage <500MB for large operations
- Rate limiting prevents Vault overload
- Concurrent operations scale linearly
- Streaming handles arbitrarily large files
- Retry logic recovers from transient failures

### 9.3 Usability Success
- Clear, intuitive CLI interface
- Comprehensive documentation
- Helpful error messages
- Progress indicators for all operations
- Easy configuration management

### 9.4 Reliability Success
- No data corruption
- Idempotent operations
- Graceful error handling
- Resumable operations
- Audit trail of all actions

### 9.5 Security Success
- Secure credential handling
- Proper permission validation
- No sensitive data in logs
- Compliance-ready audit logs

---

## 10. Risk Assessment

### 10.1 Technical Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Vault API rate limiting | High | Medium | Adaptive rate limiting, respect 429 responses |
| Large audit log processing | High | High | Streaming parser, chunked processing, checkpoints |
| Network failures during deletion | High | Medium | Retry logic, resumable operations, plan tracking |
| Memory exhaustion | Medium | Low | Stream data, limit buffer sizes, profile memory |
| Data corruption | High | Low | Atomic writes, checksums, validation |

### 10.2 Operational Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Accidental deletion of active secrets | Critical | Medium | Dry-run mode, confirmations, exclusions, Vault snapshots |
| Insufficient Vault permissions | Medium | High | Permission validation, clear errors, documentation |
| Audit log gaps | Medium | Medium | Warn on unknown access, opt-in deletion |
| Long-running operations interrupted | Medium | Medium | Resumable operations, checkpoints, progress tracking |
| Configuration errors | Medium | High | Validation command, schema validation, examples |

### 10.3 Security Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Token exposure in logs | High | Low | Redact tokens, secure logging, no debug credentials |
| Unauthorized access to data files | Medium | Medium | File permissions (0600), optional encryption |
| Privilege escalation | High | Low | Least privilege, permission validation |
| Audit trail tampering | Medium | Low | Immutable logs, checksums, external shipping |

---

## Appendix A: Vault Permissions Required

### Minimum Required Policies

**Discovery Operations**:
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

path "+/kv/metadata/*" {
  capabilities = ["list"]
}
```

**Deletion Operations**:
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

---

## Appendix B: Example Workflows

### Workflow 1: First-Time Cleanup

```bash
# 1. Validate setup
vault-secrets-cleanup validate \
  --check-permissions \
  --check-connectivity

# 2. Discover secrets
vault-secrets-cleanup discover \
  --output inventory.pb \
  --verbose

# 3. Analyze audit logs
vault-secrets-cleanup analyze \
  --audit-log "/var/log/vault/audit-*.log" \
  --inventory inventory.pb \
  --output access.pb

# 4. Create deletion plan
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-period 365d \
  --output deletion-plan.pb

# 5. Review plan
vault-secrets-cleanup report \
  --plan deletion-plan.pb \
  --type detailed \
  --format markdown \
  --output review.md

# 6. Execute
vault-secrets-cleanup execute \
  --plan deletion-plan.pb \
  --rate-limit 5
```
### Workflow 2: Using External Access Data from Log Management

```bash
# 1. Validate setup
vault-secrets-cleanup validate \
  --check-permissions \
  --check-connectivity

# 2. Discover secrets
vault-secrets-cleanup discover \
  --output inventory.pb

# 3. Import access data from external source (e.g., Splunk, Elasticsearch)
# External JSON format:
# [
#   {
#     "namespace_path": "prod/app1",
#     "mount_path": "secret",
#     "mount_accessor": "kv_xyz789",
#     "secret_path": "database/credentials",
#     "timestamp": "2024-01-15T10:30:00Z"
#   }
# ]
vault-secrets-cleanup import \
  --input external-access-data.json \
  --output access.pb

# 4. Create deletion plan with matching strategy
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-period 365d \
  --matching-strategy strict \
  --output deletion-plan.pb

# 5. Review plan
vault-secrets-cleanup report \
  --plan deletion-plan.pb \
  --format markdown

# 6. Execute
vault-secrets-cleanup execute \
  --plan deletion-plan.pb \
  --rate-limit 5
```


### Workflow 3: Incremental Cleanup

```bash
# 1. Update inventory (incremental)
vault-secrets-cleanup discover \
  --output inventory.pb \
  --incremental

# 2. Update access data (incremental)
vault-secrets-cleanup analyze \
  --audit-log "/var/log/vault/audit-$(date +%Y-%m).log" \
  --inventory inventory.pb \
  --output access.pb \
  --incremental

# 3. Create new plan
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-period 365d \
  --output deletion-plan-$(date +%Y-%m-%d).pb

# 4. Execute
vault-secrets-cleanup execute \
  --plan deletion-plan-$(date +%Y-%m-%d).pb