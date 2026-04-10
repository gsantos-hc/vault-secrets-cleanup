# Example: Basic Cleanup

## Scenario

A single team namespace with one KV v2 mount and a one-year staleness policy.

## Steps

```bash
vault-secrets-cleanup discover --output inventory.pb
vault-secrets-cleanup analyze --audit-log "audit-*.log" --output access.pb
vault-secrets-cleanup plan --inventory inventory.pb --access-data access.pb --staleness-period 365d --output plan.pb
vault-secrets-cleanup report --plan plan.pb --format markdown --output plan.md
vault-secrets-cleanup execute --plan plan.pb --dry-run --yes
```

## Outcome

`plan.md` is reviewed by platform owners before live execution.
