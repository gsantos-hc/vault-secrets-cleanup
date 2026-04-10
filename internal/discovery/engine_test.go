package discovery

import (
	"context"
	"testing"

	api "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

type fakeVaultClient struct {
	address     string
	namespaces  []string
	namespaceID string
	mounts      map[string]*api.MountOutput
	listFn      func(path string) (*api.Secret, error)
}

func (f *fakeVaultClient) Address() string { return f.address }
func (f *fakeVaultClient) ListNamespaces(ctx context.Context) ([]string, error) {
	return f.namespaces, nil
}
func (f *fakeVaultClient) WithNamespace(namespace string) (VaultClient, error) {
	clone := *f
	clone.namespaceID = namespace
	return &clone, nil
}
func (f *fakeVaultClient) ListMounts(ctx context.Context) (map[string]*api.MountOutput, error) {
	return f.mounts, nil
}
func (f *fakeVaultClient) ListSecrets(ctx context.Context, p string) (*api.Secret, error) {
	if f.listFn != nil {
		return f.listFn(p)
	}
	return nil, nil
}

func TestEngineDiscover_BuildsInventoryAndStats(t *testing.T) {
	fc := &fakeVaultClient{
		address:    "https://vault.example.com",
		namespaces: []string{"team-a/"},
		mounts: map[string]*api.MountOutput{
			"secret/": {
				Type:     "kv",
				Accessor: "kv_123",
				Options:  map[string]string{"version": "2"},
			},
		},
		listFn: func(path string) (*api.Secret, error) {
			if path == "secret/metadata" {
				return &api.Secret{Data: map[string]any{"keys": []any{"app/", "top"}}}, nil
			}
			if path == "secret/metadata/app" {
				return &api.Secret{Data: map[string]any{"keys": []any{"nested"}}}, nil
			}
			return &api.Secret{Data: map[string]any{"keys": []any{}}}, nil
		},
	}

	eng := NewEngine(Config{Client: fc})
	inv, err := eng.Discover(context.Background())
	require.NoError(t, err)
	require.Equal(t, "https://vault.example.com", inv.VaultAddress)
	require.Equal(t, int32(2), inv.Stats.NamespaceCount)
	require.Equal(t, int32(2), inv.Stats.MountCount)
	require.Equal(t, int32(4), inv.Stats.SecretCount)
	require.Equal(t, int32(4), inv.Stats.KvV2Count)
}

func TestNewEngine_Defaults(t *testing.T) {
	eng := NewEngine(Config{Client: &fakeVaultClient{address: "x"}})
	require.Equal(t, 10, eng.workers)
	require.NotNil(t, eng.rateLimiter)
}
