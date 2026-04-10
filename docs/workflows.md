# Workflows

## Full Cleanup Workflow

1. Validate connectivity and permissions.
2. Run `discover` and produce `inventory.pb`.
3. Run `analyze` against one or more audit logs.
4. Run `plan` with staleness and exclusion settings.
5. Run `report` in markdown and JSON for review.
6. Run `execute --dry-run` and validate expected actions.
7. Run `execute` live after explicit approval.

## External Access Data Workflow

1. Export access data from SIEM or observability tooling.
2. Convert to supported JSON records.
3. Run `import --input external.json --output access.pb`.
4. Continue with `plan`, `report`, and `execute`.

## Incremental Cleanup Workflow

1. Start with narrow namespace/mount scope.
2. Apply strict exclusion patterns.
3. Execute dry-run and live run for pilot scope.
4. Expand scope gradually across environments.
5. Keep prior plan files for audit traceability.

## Security Workflow

1. Use least-privilege Vault token for each stage.
2. Keep `VAULT_TOKEN` in environment only.
3. Rotate tokens after cleanup windows.
4. Store generated `.pb` files in restricted locations.
5. Remove temporary artifacts after run completion.
