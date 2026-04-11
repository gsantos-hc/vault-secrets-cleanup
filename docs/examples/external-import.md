# Example: External Import

## Scenario

Audit logs are retained in a SIEM platform, not on the Vault host.

## Steps

1. Export access records into the expected JSON shape.
2. Import data:

```bash
vault-secrets-cleanup import --input access-export.json --output access.pb
```

3. Continue with plan/report/execute:

```bash
vault-secrets-cleanup plan --inventory inventory.pb --access-data access.pb --output plan.pb
vault-secrets-cleanup report --plan plan.pb --format json --output plan.json
```

## Outcome

Cleanup planning can proceed without direct access to raw audit log files.
