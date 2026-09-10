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
  --seed 42 \
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

The required capabilities depend on which setup stages are executed. Secret writes are always required. Namespace and mount creation are only required when the corresponding `--skip-*` flag is **not** set. When `--skip-mounts` is used, the engine reads `sys/mounts` in every namespace to discover the real KV version of each existing mount.

**Always required** (secret writes):

```hcl
# Write KV v2 secrets: <namespace>/<mount>/data/<secret>
path "+/+/data/+" {
  capabilities = ["create", "update"]
}

# Write KV v1 secrets: <namespace>/<mount>/<secret>
path "+/+/+" {
  capabilities = ["create", "update"]
}
```

**Required unless `--skip-mounts` is set** (mount enablement):

```hcl
path "+/sys/mounts/*" {
  capabilities = ["create", "update"]
}
```

**Required when `--skip-mounts` is set** (mount version discovery):

```hcl
path "+/sys/mounts" {
  capabilities = ["read"]
}
```

**Required unless `--skip-namespaces` is set** (namespace creation):

```hcl
path "sys/namespaces/*" {
  capabilities = ["create", "update"]
}
```
