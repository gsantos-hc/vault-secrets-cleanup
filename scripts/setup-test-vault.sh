#!/usr/bin/env bash
# Copyright IBM Corp. 2026
# SPDX-License-Identifier: MIT

set -euo pipefail

if ! command -v vault >/dev/null 2>&1; then
  echo "vault CLI is required but was not found in PATH" >&2
  exit 1
fi

if lsof -Pi :8200 -sTCP:LISTEN -t >/dev/null 2>&1; then
  echo "port 8200 is already in use; stop the existing Vault process first" >&2
  exit 1
fi

echo "Starting Vault in dev mode..."
vault server -dev -dev-root-token-id=root >/tmp/vault-secrets-cleanup-dev.log 2>&1 &
VAULT_PID=$!

cleanup() {
  if kill -0 "$VAULT_PID" >/dev/null 2>&1; then
    kill "$VAULT_PID" >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

export VAULT_ADDR="http://127.0.0.1:8200"
export VAULT_TOKEN="root"

for _ in $(seq 1 20); do
  if vault status >/dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

vault status >/dev/null 2>&1

echo "Creating test namespaces..."
vault namespace create prod >/dev/null
vault namespace create dev >/dev/null

echo "Enabling KV engines..."
vault secrets enable -path=secret kv-v2 >/dev/null
vault secrets enable -path=kv kv >/dev/null

echo "Creating test secrets..."
vault kv put secret/app1/config username=admin password=secret >/dev/null
vault kv put secret/app2/config api_key=12345 >/dev/null

echo
echo "Test Vault ready"
echo "VAULT_ADDR=$VAULT_ADDR"
echo "VAULT_TOKEN=$VAULT_TOKEN"
echo "VAULT_PID=$VAULT_PID"
echo
echo "Run integration tests in another shell with:"
echo "  export VAULT_TEST_ADDR=$VAULT_ADDR"
echo "  export VAULT_TEST_TOKEN=$VAULT_TOKEN"
echo "  go test -tags=integration -v ./tests/integration/..."
echo
echo "Press Ctrl+C to stop Vault."
wait "$VAULT_PID"
