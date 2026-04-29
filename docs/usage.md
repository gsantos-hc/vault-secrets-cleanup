# Usage

## Core Commands

### Discover

```bash
vault-secrets-cleanup discover --output inventory.pb
```

### Analyze

```bash
vault-secrets-cleanup analyze \
  --audit-log "/var/log/vault/audit-*.log" \
  --output access.pb
```

### Plan

```bash
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-period 365d \
  --output deletion-plan.pb
```

### Report

```bash
vault-secrets-cleanup report \
  --plan deletion-plan.pb \
  --format markdown
```

### Execute (Dry Run)

```bash
vault-secrets-cleanup execute \
  --plan deletion-plan.pb \
  --dry-run \
  --yes
```

### Execute (Live)

```bash
vault-secrets-cleanup execute \
  --plan deletion-plan.pb \
  --workers 10 \
  --rate-limit 5 \
  --circuit-breaker 10
```

### Execute Worker Controls

- Configure default worker count in config: `parallel.workers: 10`
- Override per run with CLI flag: `--workers <n>`
- Not all workflows use parallel workers yet; currently `execute` is the active adopter.

## Safety Checklist

- Create a Vault snapshot before any deletion run.
- Always run `execute --dry-run` first.
- Review report output with platform/security owners.
- Start with conservative rate limits.
