# Example: External Import

## Scenario

Audit logs are retained in a SIEM platform, not on the Vault host.

## Steps

1. Export access records into the expected JSON shape.

The importer accepts either:

- A JSON array of pre-aggregated access records (`namespace_path`, `mount_path`, `mount_accessor`, `secret_path`, `last_accessed`, `access_type`, `access_count`).
- A JSON array from `docs/queries/kvv2_secrets.jq` (`namespace_path`, `mount_accessor`, `mount_path`, `secret_path`, `timestamp`).

For KVv2 extractor data, the importer normalizes paths before aggregating:

- Strips embedded namespace prefixes from `mount_path` when present.
- Detects KVv2 API path segments (`data`, `metadata`, `subkeys`) in `secret_path`.
- Groups records by normalized `mount_accessor` + `secret_path` and computes `access_count` and latest `last_accessed`.

2. Import data:

```bash
vault-secrets-cleanup import --input access-export.json --output access.pb
```

3. Continue with plan/report/execute:

```bash
vault-secrets-cleanup plan --inventory inventory.pb --access-data access.pb --output plan.pb
vault-secrets-cleanup report --plan plan.pb --format json --output plan.json
```

If your imported data does not contain reliable `mount_accessor` values, fall back to path-based matching:

```bash
vault-secrets-cleanup plan --inventory inventory.pb --access-data access.pb --matching-strategy path-based --output plan.pb
```

## Outcome

Cleanup planning can proceed without direct access to raw audit log files.
