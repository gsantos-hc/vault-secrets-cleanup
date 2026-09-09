// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

//go:build integration

package vault

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClientIntegration_HealthAndNamespaces(t *testing.T) {
	addr := os.Getenv("VAULT_TEST_ADDR")
	token := os.Getenv("VAULT_TEST_TOKEN")
	if addr == "" || token == "" {
		t.Skip("set VAULT_TEST_ADDR and VAULT_TEST_TOKEN to run integration test")
	}

	client, err := NewClient(Config{Address: addr, Token: token})
	require.NoError(t, err)

	health, err := client.Health(context.Background())
	require.NoError(t, err)
	require.True(t, health.Initialized)
}
