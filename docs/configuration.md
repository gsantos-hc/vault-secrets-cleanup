# Configuration

Configuration is loaded from the following sources, in precedence order:

1. CLI flags
2. Environment variables
3. YAML config file
4. Defaults

## Example

```yaml
vault:
  address: https://vault.example.com
  token: ${VAULT_TOKEN}

rate_limit:
  requests_per_second: 100
  burst: 100

staleness:
  default_period: 365d
  namespaces:
    prod/*: 730d
    dev/*: 90d

exclusions:
  namespaces:
    - admin/*
  paths:
    - "*/bootstrap/*"
  mounts:
    - "identity/*"

logging:
  level: info
  format: text
```

## Environment Variables

- `VAULT_ADDR`: Vault API address
- `VAULT_TOKEN`: Vault token used by CLI commands

## Important Notes

- `vault.address` and `vault.token` are required.
- `staleness.default_period` must be a positive duration like `365d`, `720h`, or `30m`.
- Use exclusion rules for critical namespaces and bootstrap paths.
