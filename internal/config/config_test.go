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

func TestValidate_RequiredFields(t *testing.T) {
	cfg := Config{}
	err := cfg.Validate()
	require.Error(t, err)
}
