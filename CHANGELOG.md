# Changelog

All notable changes to this project are documented in this file.

## [1.0.0] - 2026-04-10

### Added

- Integration tests for audit pipeline and planning pipeline.
- End-to-end workflow test covering analyze, plan, execute dry-run, and report.
- Benchmark tests for audit parsing and discovery stats calculation.
- Benchmark runner script at `scripts/benchmark.sh`.
- Local Vault setup helper for integration tests at `scripts/setup-test-vault.sh`.
- CI workflow for formatting, vet, and test execution.
- Release workflow for multi-platform binaries and checksums.
- New operational documentation:
  - Installation guide
  - Configuration reference
  - Usage guide
  - Workflow guide
  - Troubleshooting guide
  - Example scenarios

### Changed

- Makefile now includes dedicated targets for integration tests, e2e tests, and benchmarks.
- README now links to full documentation set and release-readiness commands.
