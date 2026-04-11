# Example: Incremental Cleanup

## Scenario

Production rollout with strict safety and phased execution.

## Phase 1: Pilot Namespace

```bash
vault-secrets-cleanup plan \
  --inventory inventory.pb \
  --access-data access.pb \
  --staleness-period 730d \
  --output prod-pilot-plan.pb
```

Run report and dry-run, then execute live with low rate limit.

## Phase 2: Broader Scope

- Expand namespace coverage.
- Keep exclusion rules for critical paths.
- Re-run dry-run before each live execution.

## Outcome

Operational risk is reduced by limiting blast radius at each stage.
