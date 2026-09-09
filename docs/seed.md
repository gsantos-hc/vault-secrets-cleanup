# Cluster Seeding Script

Use the standalone seeding script to generate synthetic Vault Enterprise data with:

- configurable namespace count,
- randomized mount distribution per namespace,
- randomized secret distribution per mount,
- exact user-defined total secret count,
- parallel workers within each seeding stage.

The script runs in three ordered stages: it creates namespaces first, then enables mounts across all namespaces, then writes secrets across all mounts. The `--workers` flag controls how many operations can run in parallel within each stage.

```bash
go run ./scripts/seed \
  --config example-config.yaml \
  --namespaces 25 \
  --total-secrets 50000 \
  --workers 8 \
  --kv2-probability 0.9 \
  --namespace-prefix seed \
  --mount-prefix seed \
  --seed 42
```

Dry run example (no writes):

```bash
go run ./scripts/seed \
  --config example-config.yaml \
  --namespaces 10 \
  --total-secrets 1000 \
  --workers 4 \
  --dry-run
```

Resume a partial run by skipping setup stages that already completed:

```bash
go run ./scripts/seed \
  --config example-config.yaml \
  --namespaces 25 \
  --total-secrets 50000 \
  --workers 8 \
  --skip-namespaces \
  --skip-mounts
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--config` | _(required)_ | Path to Vault config file |
| `--namespaces` | `10` | Number of namespaces to create |
| `--total-secrets` | `1000` | Total number of secrets to write |
| `--workers` | `4` | Parallel workers per stage |
| `--kv2-probability` | `0.9` | Fraction of mounts to create as KV v2 |
| `--namespace-prefix` | `seed` | Prefix for generated namespace names |
| `--mount-prefix` | `seed` | Prefix for generated mount names |
| `--seed` | _(random)_ | Random seed for reproducible runs |
| `--skip-namespaces` | `false` | Skip namespace creation (assume they already exist) |
| `--skip-mounts` | `false` | Skip mount enablement (assume they already exist) |
| `--dry-run` | `false` | Plan all operations without writing to Vault |
| `--max-failures` | `0` | Failed-operation tolerance: `0` = stop on first, `N` = stop after N, `-1` = never stop |
| `--max-attempts` | `3` | Write retry attempts per operation |
| `--progress` | `auto` | Progress output mode: `auto`, `tty`, `log`, or `off` |

## Required Vault capabilities

Required Vault capabilities for seeding include namespace creation, mount creation, and secret writes:

```hcl
path "sys/namespaces/*" {
  capabilities = ["create", "update"]
}

path "+/sys/mounts/*" {
  capabilities = ["create", "update"]
}

path "+/*/data/*" {
  capabilities = ["create", "update"]
}

path "+/*" {
  capabilities = ["create", "update"]
}
```
