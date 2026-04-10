package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	api "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

type fakeValidationClient struct {
	healthErr error
	listErr   error
}

func (f *fakeValidationClient) Health(ctx context.Context) (*api.HealthResponse, error) {
	if f.healthErr != nil {
		return nil, f.healthErr
	}
	return &api.HealthResponse{Initialized: true, Sealed: false}, nil
}

func (f *fakeValidationClient) ListNamespaces(ctx context.Context) ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return []string{"team-a/"}, nil
}

func baseCfg() *config.Config {
	return &config.Config{
		Vault:     config.VaultConfig{Address: "https://vault.example.com", Token: "token"},
		RateLimit: config.RateLimitConfig{RequestsPerSecond: 10},
		Staleness: config.StalenessConfig{DefaultPeriod: "365d"},
	}
}

func TestRunValidate_Success(t *testing.T) {
	old := newValidationClient
	newValidationClient = func(cfg *config.Config) (validationClient, error) {
		return &fakeValidationClient{}, nil
	}
	t.Cleanup(func() { newValidationClient = old })

	err := runValidate(context.Background(), baseCfg(), validateOptions{checkConnectivity: true, checkPermissions: true})
	require.NoError(t, err)
}

func TestRunValidate_ConnectivityFailure(t *testing.T) {
	old := newValidationClient
	newValidationClient = func(cfg *config.Config) (validationClient, error) {
		return &fakeValidationClient{healthErr: errors.New("down")}, nil
	}
	t.Cleanup(func() { newValidationClient = old })

	err := runValidate(context.Background(), baseCfg(), validateOptions{checkConnectivity: true})
	require.Error(t, err)
}

func TestRunValidate_PermissionsFailure(t *testing.T) {
	old := newValidationClient
	newValidationClient = func(cfg *config.Config) (validationClient, error) {
		return &fakeValidationClient{listErr: errors.New("forbidden")}, nil
	}
	t.Cleanup(func() { newValidationClient = old })

	err := runValidate(context.Background(), baseCfg(), validateOptions{checkPermissions: true})
	require.Error(t, err)
}
