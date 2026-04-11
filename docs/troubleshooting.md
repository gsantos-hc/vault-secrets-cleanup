# Troubleshooting

## `vault.address is required`

Set one of:

- `--vault-addr` flag
- `VAULT_ADDR` environment variable
- `vault.address` in config file

## `vault.token is required`

Set one of:

- `--vault-token` flag (avoid in shell history)
- `VAULT_TOKEN` environment variable
- `vault.token` in config file

## No audit files found

The `analyze` command supports glob patterns. Confirm your shell expansion and file path:

```bash
ls /var/log/vault/audit-*.log
```

## Discovery permission errors

Grant token capabilities for namespace listing, mount reads, and secret metadata listing.

## Execute aborted by circuit breaker

Too many consecutive deletion failures were observed.

- Inspect failure causes in logs.
- Lower rate limit.
- Fix permission or path issues.
- Re-run `execute` to resume remaining pending items.

## Integration tests skipped

Set required env vars before running integration tests:

```bash
export VAULT_TEST_ADDR=http://127.0.0.1:8200
export VAULT_TEST_TOKEN=root
go test -tags=integration -v ./tests/integration/...
```
