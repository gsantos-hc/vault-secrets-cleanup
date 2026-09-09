// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoad_FromFile(t *testing.T) {
	t.Setenv("VAULT_ADDR", "")
	t.Setenv("VAULT_TOKEN", "")

	cfgPath := writeTempConfig(t, `vault:
  address: https://vault.file.example.com
  token: file-token
rate_limit:
  requests_per_second: 42
`)

	cfg, err := Load(cfgPath, nil)
	require.NoError(t, err)
	require.Equal(t, "https://vault.file.example.com", cfg.Vault.Address)
	require.Equal(t, "file-token", cfg.Vault.Token)
	require.Equal(t, 42.0, cfg.RateLimit.RequestsPerSecond)
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.env.example.com")
	t.Setenv("VAULT_TOKEN", "env-token")

	cfgPath := writeTempConfig(t, `vault:
  address: https://vault.file.example.com
  token: file-token
`)

	cfg, err := Load(cfgPath, nil)
	require.NoError(t, err)
	require.Equal(t, "https://vault.env.example.com", cfg.Vault.Address)
	require.Equal(t, "env-token", cfg.Vault.Token)
}

func TestLoad_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.env.example.com")
	t.Setenv("VAULT_TOKEN", "env-token")

	cfgPath := writeTempConfig(t, `vault:
  address: https://vault.file.example.com
  token: file-token
`)

	cfg, err := Load(cfgPath, map[string]any{
		"vault.address": "https://vault.flag.example.com",
		"vault.token":   "flag-token",
	})
	require.NoError(t, err)
	require.Equal(t, "https://vault.flag.example.com", cfg.Vault.Address)
	require.Equal(t, "flag-token", cfg.Vault.Token)
}

func TestLoad_EmptyFlagsDoNotOverrideEnv(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.env.example.com")
	t.Setenv("VAULT_TOKEN", "env-token")

	cfg, err := Load("", map[string]any{
		"vault.address": "",
		"vault.token":   "",
	})
	require.NoError(t, err)
	require.Equal(t, "https://vault.env.example.com", cfg.Vault.Address)
	require.Equal(t, "env-token", cfg.Vault.Token)
}

func TestValidate_RequiredFields(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	require.Error(t, err)
}

func TestLoad_StalenessPeriodFromFile(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.env.example.com")
	t.Setenv("VAULT_TOKEN", "env-token")

	cfgPath := writeTempConfig(t, `staleness:
  default_period: 365d
  namespaces:
    prod/*: 1d
    dev/*: 5m
`)

	cfg, err := Load(cfgPath, nil)
	require.NoError(t, err)
	require.Equal(t, "365d", cfg.Staleness.DefaultPeriod)
	require.Equal(t, "1d", cfg.Staleness.Namespaces["prod/*"])
	require.Equal(t, "5m", cfg.Staleness.Namespaces["dev/*"])
}

func TestValidate_InvalidStalenessPeriod(t *testing.T) {
	cfg := Config{
		Vault:     VaultConfig{Address: "https://vault.example.com", Token: "token"},
		RateLimit: RateLimitConfig{RequestsPerSecond: 100},
		Staleness: StalenessConfig{DefaultPeriod: "bad"},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "staleness.default_period")
}

func TestLoad_DefaultParallelWorkers(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.env.example.com")
	t.Setenv("VAULT_TOKEN", "env-token")

	cfg, err := Load("", nil)
	require.NoError(t, err)
	require.Equal(t, 10, cfg.Parallel.Workers)
}

func TestLoad_ParallelWorkersFromFile(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.env.example.com")
	t.Setenv("VAULT_TOKEN", "env-token")

	cfgPath := writeTempConfig(t, `parallel:
  workers: 7
`)

	cfg, err := Load(cfgPath, nil)
	require.NoError(t, err)
	require.Equal(t, 7, cfg.Parallel.Workers)
}

func TestValidate_InvalidParallelWorkers(t *testing.T) {
	cfg := Config{
		Vault:     VaultConfig{Address: "https://vault.example.com", Token: "token"},
		RateLimit: RateLimitConfig{RequestsPerSecond: 100},
		Staleness: StalenessConfig{DefaultPeriod: "365d"},
		Parallel:  ParallelConfig{Workers: 0},
	}

	err := cfg.Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "parallel.workers")
}

func TestLoad_ParallelWorkersFlagsOverrideFile(t *testing.T) {
	t.Setenv("VAULT_ADDR", "https://vault.env.example.com")
	t.Setenv("VAULT_TOKEN", "env-token")

	cfgPath := writeTempConfig(t, `parallel:
  workers: 7
`)

	cfg, err := Load(cfgPath, map[string]any{"parallel.workers": 3})
	require.NoError(t, err)
	require.Equal(t, 3, cfg.Parallel.Workers)
}
