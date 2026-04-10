//go:build integration

package integration

import (
	"context"
	"os"
	"testing"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/discovery"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	"github.com/stretchr/testify/require"
)

func TestDiscoveryIntegration(t *testing.T) {
	addr := os.Getenv("VAULT_TEST_ADDR")
	token := os.Getenv("VAULT_TEST_TOKEN")
	if addr == "" || token == "" {
		t.Skip("set VAULT_TEST_ADDR and VAULT_TEST_TOKEN to run integration test")
	}

	client, err := vaultpkg.NewClient(vaultpkg.Config{Address: addr, Token: token})
	require.NoError(t, err)

	engine := discovery.NewEngine(discovery.Config{
		Client:      discovery.NewVaultClientAdapter(client),
		RateLimiter: ratelimit.New(10),
		Workers:     5,
	})

	inv, err := engine.Discover(context.Background())
	require.NoError(t, err)
	require.NotNil(t, inv)
	require.NotNil(t, inv.Stats)
}
