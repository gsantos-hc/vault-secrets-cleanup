package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/discovery"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

type fakeDiscoverEngine struct {
	inventory *vpb.Inventory
	err       error
}

func (f *fakeDiscoverEngine) Discover(ctx context.Context) (*vpb.Inventory, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.inventory, nil
}

func TestRunDiscover_SuccessWritesFile(t *testing.T) {
	oldCfg := loadedCfg
	loadedCfg = &config.Config{
		Vault:     config.VaultConfig{Address: "https://vault.example.com", Token: "token"},
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 10},
		Staleness: config.StalenessConfig{DefaultPeriod: "365d"},
	}
	t.Cleanup(func() { loadedCfg = oldCfg })

	oldFactory := createDiscoverEngineWithProgress
	createDiscoverEngineWithProgress = func(cfg *config.Config, workers int, _ func(discovery.ProgressSnapshot)) (discoverEngine, error) {
		return &fakeDiscoverEngine{inventory: &vpb.Inventory{VaultAddress: cfg.Vault.Address, Stats: &vpb.InventoryStats{NamespaceCount: 1}}}, nil
	}
	t.Cleanup(func() { createDiscoverEngineWithProgress = oldFactory })

	output := filepath.Join(t.TempDir(), "inventory.pb")
	err := runDiscover(context.Background(), discoverOptions{output: output, workers: 2})
	require.NoError(t, err)

	info, statErr := os.Stat(output)
	require.NoError(t, statErr)
	require.Greater(t, info.Size(), int64(0))
}

func TestRunDiscover_FailsOnEngineError(t *testing.T) {
	oldCfg := loadedCfg
	loadedCfg = &config.Config{
		Vault:     config.VaultConfig{Address: "https://vault.example.com", Token: "token"},
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 10},
		Staleness: config.StalenessConfig{DefaultPeriod: "365d"},
	}
	t.Cleanup(func() { loadedCfg = oldCfg })

	oldFactory := createDiscoverEngineWithProgress
	createDiscoverEngineWithProgress = func(cfg *config.Config, workers int, _ func(discovery.ProgressSnapshot)) (discoverEngine, error) {
		return &fakeDiscoverEngine{err: errors.New("boom")}, nil
	}
	t.Cleanup(func() { createDiscoverEngineWithProgress = oldFactory })

	err := runDiscover(context.Background(), discoverOptions{output: filepath.Join(t.TempDir(), "inventory.pb"), workers: 1})
	require.Error(t, err)
}
