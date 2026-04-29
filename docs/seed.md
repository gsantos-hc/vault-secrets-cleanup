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
