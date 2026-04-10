# Vault Secrets Cleanup Tool - Executive Summary

## Overview

This document provides a high-level summary of the requirements for a Go-based CLI tool that helps identify and remove stale secrets from HashiCorp Vault Enterprise.

## Key Recommendations

### 1. Data Format: Protocol Buffers over JSON

**Recommendation**: Use Protocol Buffers (protobuf) for internal data files instead of JSON.

**Benefits**:
- **3-10x smaller** file sizes (critical for large inventories)
- **5-10x faster** parsing/serialization (important for 10GB+ audit logs)
- **Built-in schema validation** prevents data corruption
- **Excellent streaming support** for large-scale processing
- **Type safety** reduces bugs

**Implementation**:
- Use protobuf for: inventory files, access data, deletion plans
- Use JSONL for: human-readable reports, debugging
- Use YAML for: configuration files

### 2. Architecture: Modular Design

```
CLI Layer (cobra/viper)
    ↓
Application Layer (Discovery, Audit Processing, Deletion)
    ↓
Service Layer (Vault Client, Storage, Rate Limiter)
    ↓
Infrastructure Layer (Vault API, File System, Logging)
```

**Key Components**:
- **Discovery Engine**: Enumerate namespaces, mounts, and secrets
- **Audit Processor**: Stream and parse large audit logs efficiently
- **Correlation Engine**: Match inventory with access patterns
- **Deletion Engine**: Execute plans with safety features

### 3. Performance Targets

| Metric | Target | Strategy |
|--------|--------|----------|
| Audit log processing | 10GB in <30 min | Streaming parser, parallel processing |
| Memory usage | <500MB | Stream data, no full file loads |
| API rate limiting | 100 req/s default | Configurable, adaptive |
| Secrets discovery | 100k+ secrets | Pagination, concurrent workers |
| Deletion throughput | 10k secrets | Rate-limited, resumable |

### 4. Safety Features

**Critical safeguards**:
1. **Dry-run mode**: Generate plans without executing
2. **Confirmation prompts**: Require explicit approval before deletion
3. **Circuit breaker**: Stop after N consecutive failures
4. **Exclusion rules**: Protect critical secrets (bootstrap, root tokens)
5. **Unknown-access opt-in**: Require explicit flag to delete secrets with no audit trail
6. **Resumable operations**: Continue after interruption
7. **Hard delete**: Permanently remove secrets from storage (KV v1 and v2)

**IMPORTANT**: Operators must take Vault snapshots before running deletions. This tool performs permanent deletions and does not create backups.

### 5. CLI Command Structure

```bash
vault-secrets-cleanup
├── discover   # Create secrets inventory
├── analyze    # Parse audit logs for access patterns
├── import     # Import access data from external sources
├── plan       # Generate deletion plan
├── execute    # Execute approved plan
├── report     # Generate reports
└── validate   # Validate setup and permissions
```

**Typical workflow**:
```bash
# 1. Discover all secrets
vault-secrets-cleanup discover --output inventory.pb

# 2a. Analyze audit logs (option 1)
vault-secrets-cleanup analyze \
  --audit-log "/var/log/vault/audit-*.log" \
  --inventory inventory.pb \
  --output access.pb

# 2b. Import from external source (option 2)
vault-secrets-cleanup import \
  --input external-access-data.json \
  --output access.pb

# 3. Create deletion plan
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-days 365 \
  --matching-strategy strict \
  --output deletion-plan.pb

# 4. Review plan
vault-secrets-cleanup report \
  --plan deletion-plan.pb \
  --format markdown

# 5. Execute (ensure Vault snapshot taken first!)
vault-secrets-cleanup execute \
  --plan deletion-plan.pb \
  --rate-limit 5
```

### 6. Additional Recommended Features

Beyond your initial requirements, the following features are included:

1. **External access data import**: Import access timestamps from log management solutions (Splunk, Elasticsearch, etc.)
2. **Configurable matching strategies**:
  - Strict: Match by namespace path + mount accessor + secret path
  - Path-based: Match by namespace path + mount path + secret path
  - Hybrid: Try strict first, fall back to path-based
4. **Incremental operations**: Update inventory/access data without full re-scan
2. **Per-namespace staleness**: Different retention periods for prod vs dev
3. **Compression support**: Handle gzip/bzip2 audit logs
4. **Multi-file processing**: Merge data from multiple audit log files
5. **Progress tracking**: Real-time progress bars with ETA
6. **Resumable deletions**: Continue from checkpoint after interruption
7. **Access pattern analytics**: Identify trends and anomalies
8. **GitOps integration**: Version control deletion plans for approval workflows
9. **Notification webhooks**: Alert on completion/failures (Slack, Teams)
10. **Metrics export**: Prometheus metrics for monitoring

### 7. Configuration Management

**Three-tier configuration**:
1. **Configuration file** (YAML): Base settings
2. **Environment variables**: Override for CI/CD
3. **CLI flags**: Highest priority, per-command overrides

**Example config**:
```yaml
vault:
  address: https://vault.example.com
  auth_method: token

rate_limit:
  discovery: 100
  deletion: 10

staleness:
  default_days: 365
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

### 8. Implementation Timeline

**13-week roadmap** (3 months):

- **Weeks 1-2**: Foundation (CLI, config, Vault client, rate limiting)
- **Weeks 3-4**: Discovery engine (namespace/mount/secret enumeration)
- **Weeks 5-7**: Audit processing (streaming parser, large-scale testing)
- **Weeks 8-9**: Correlation & planning (staleness calculation, exclusions)
- **Weeks 10-11**: Deletion engine (execution, safety features)
- **Weeks 12-13**: Polish & release (testing, docs, v1.0.0)

**Post-v1.0 enhancements**:
- Multi-cluster support
- Advanced analytics and reporting
- Notification system
- GitOps integration
- Metrics and monitoring

### 9. Success Metrics

**Functional**:
- ✓ Discover 100k+ secrets across namespaces
- ✓ Process 10GB audit log in <30 minutes
- ✓ Delete 10k secrets with <1% error rate

**Performance**:
- ✓ Memory usage <500MB for large operations
- ✓ Rate limiting prevents Vault overload
- ✓ Streaming handles arbitrarily large files

**Reliability**:
- ✓ No data corruption
- ✓ Idempotent operations
- ✓ Graceful error handling
- ✓ Resumable operations

**Security**:
- ✓ Secure credential handling
- ✓ Permission validation
- ✓ No sensitive data in logs
- ✓ Audit trail of all actions

### 10. Risk Mitigation

**Top risks and mitigations**:

1. **Accidental deletion of active secrets**
   - Mitigation: Dry-run mode, confirmations, exclusions, Vault snapshots (operator responsibility)

2. **Large audit log processing**
   - Mitigation: Streaming parser, chunked processing, checkpoints

3. **Vault API rate limiting**
   - Mitigation: Adaptive rate limiting, respect 429 responses

4. **Network failures during deletion**
   - Mitigation: Retry logic, resumable operations, plan tracking

5. **Audit log gaps (unknown access)**
   - Mitigation: Warn users, opt-in deletion, conservative defaults

## Next Steps

1. **Review this requirements document** and provide feedback
2. **Approve the plan** or request modifications
3. **Switch to Code mode** to begin implementation
4. **Start with Phase 1** (Foundation) - CLI framework and core infrastructure

## Questions for Consideration

1. **Audit log retention**: Recommend 13 months for accurate analysis?
2. **Notification priority**: Start with webhooks (Slack/Teams)?
3. **KV migration**: Document v1→v2 migration benefits before cleanup?

## Resources

- **Full Requirements**: See [`REQUIREMENTS.md`](REQUIREMENTS.md:1) for complete details
- **Vault API Docs**: https://developer.hashicorp.com/vault/api-docs
- **Protocol Buffers**: https://protobuf.dev/
- **Go Vault SDK**: https://github.com/hashicorp/vault/tree/main/api

---

**Ready to proceed?** Review the requirements and let me know if you'd like any changes before we move to implementation!