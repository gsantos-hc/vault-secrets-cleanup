#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

cd "$REPO_ROOT"

echo "Running benchmark suite..."
go test -run '^$' -bench=. -benchmem ./internal/audit ./internal/discovery ./internal/correlation ./internal/planning

echo
echo "Running focused workflow timing..."
if [ -x "./bin/vault-secrets-cleanup" ]; then
  time ./bin/vault-secrets-cleanup --help >/dev/null
else
  echo "Binary not found at ./bin/vault-secrets-cleanup, skipping CLI timing"
fi
