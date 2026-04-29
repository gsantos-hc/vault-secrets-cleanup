package discovery

import (
	"context"
	"sort"
	"sync"
	"sync/atomic"
	"testing"

	api "github.com/hashicorp/vault/api"
	"github.com/stretchr/testify/require"
)

type fakeVaultClient struct {
	address      string
	namespaces   []string
	namespaceID  string
	mounts       map[string]*api.MountOutput
	listMountsFn func(namespace string) (map[string]*api.MountOutput, error)
	listFn       func(path string) (*api.Secret, error)
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
	if f.listMountsFn != nil {
		return f.listMountsFn(f.namespaceID)
	}
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

func TestEngineDiscover_ListsAllMountsBeforeSecrets(t *testing.T) {
	var mountCalls int32
	var mountPhaseDone atomic.Bool
	var mountMu sync.Mutex
	seenNamespaces := map[string]bool{}

	fc := &fakeVaultClient{
		address:    "https://vault.example.com",
		namespaces: []string{"team-a/"},
		listMountsFn: func(namespace string) (map[string]*api.MountOutput, error) {
			mountMu.Lock()
			seenNamespaces[namespace] = true
			mountMu.Unlock()

			if atomic.AddInt32(&mountCalls, 1) == 2 {
				mountPhaseDone.Store(true)
			}

			return map[string]*api.MountOutput{
				"secret/": {
					Type:     "kv",
					Accessor: "kv_123",
					Options:  map[string]string{"version": "2"},
				},
			}, nil
		},
		listFn: func(path string) (*api.Secret, error) {
			if !mountPhaseDone.Load() {
				return nil, context.DeadlineExceeded
			}
			return &api.Secret{Data: map[string]any{"keys": []any{}}}, nil
		},
	}

	eng := NewEngine(Config{Client: fc, Workers: 4})
	inv, err := eng.Discover(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(2), inv.Stats.NamespaceCount)
	require.Equal(t, int32(2), inv.Stats.MountCount)
	require.Equal(t, int32(0), inv.Stats.SecretCount)

	mountMu.Lock()
	require.True(t, seenNamespaces[""])
	require.True(t, seenNamespaces["team-a/"])
	mountMu.Unlock()
}

func TestEngineDiscover_DeduplicatesAndNormalizesNestedSecrets(t *testing.T) {
	fc := &fakeVaultClient{
		address: "https://vault.example.com",
		mounts: map[string]*api.MountOutput{
			"secret/": {
				Type:     "kv",
				Accessor: "kv_123",
				Options:  map[string]string{"version": "2"},
			},
		},
		listFn: func(path string) (*api.Secret, error) {
			switch path {
			case "secret/metadata":
				return &api.Secret{Data: map[string]any{"keys": []any{"app/", "app//", "top", "top"}}}, nil
			case "secret/metadata/app":
				return &api.Secret{Data: map[string]any{"keys": []any{"nested", "nested"}}}, nil
			default:
				return &api.Secret{Data: map[string]any{"keys": []any{}}}, nil
			}
		},
	}

	eng := NewEngine(Config{Client: fc, Workers: 4})
	inv, err := eng.Discover(context.Background())
	require.NoError(t, err)
	require.Len(t, inv.Namespaces, 1)
	require.Len(t, inv.Namespaces[0].Mounts, 1)

	paths := make([]string, 0, len(inv.Namespaces[0].Mounts[0].Secrets))
	for _, secret := range inv.Namespaces[0].Mounts[0].Secrets {
		paths = append(paths, secret.Path)
	}
	sort.Strings(paths)
	require.Equal(t, []string{"app/nested", "top"}, paths)
	require.Equal(t, int32(2), inv.Stats.SecretCount)
}
