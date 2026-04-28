package seeding

import (
	"context"
	"testing"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	"github.com/stretchr/testify/require"
)

type fakeVaultWriter struct {
	namespaces []string
	mounts     []mountCall
	writes     []writeCall
}

type mountCall struct {
	namespace string
	path      string
	version   int
}

type writeCall struct {
	namespace string
	mountPath string
	path      string
	version   int
}

func (f *fakeVaultWriter) CreateNamespace(ctx context.Context, namespace string) error {
	f.namespaces = append(f.namespaces, namespace)
	return nil
}

func (f *fakeVaultWriter) EnableKVMount(ctx context.Context, namespace, mountPath string, kvVersion int) error {
	f.mounts = append(f.mounts, mountCall{namespace: namespace, path: mountPath, version: kvVersion})
	return nil
}

func (f *fakeVaultWriter) WriteKVSecret(ctx context.Context, namespace, mountPath, secretPath string, data map[string]any, kvVersion int) error {
	f.writes = append(f.writes, writeCall{namespace: namespace, mountPath: mountPath, path: secretPath, version: kvVersion})
	return nil
}

type noOpLimiter struct{}

func (n noOpLimiter) Wait(ctx context.Context) error {
	return nil
}

func TestEngineRun_CreatesExactTotalSecrets(t *testing.T) {
	fake := &fakeVaultWriter{}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 3,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RandomSeed:     123,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, result.NamespacesCreated)
	require.Equal(t, 20, result.SecretsWritten)
	require.Greater(t, result.MountsCreated, 0)
	require.Len(t, fake.namespaces, 3)
	require.Equal(t, result.MountsCreated, len(fake.mounts))
	require.Equal(t, 20, len(fake.writes))
}

func TestEngineRun_DryRunWritesNothing(t *testing.T) {
	fake := &fakeVaultWriter{}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 2,
		TotalSecrets:   10,
		KV2Probability: 0.9,
		RandomSeed:     123,
		DryRun:         true,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, result.PlannedNamespaces)
	require.Equal(t, 10, result.PlannedSecrets)
	require.Zero(t, result.NamespacesCreated)
	require.Zero(t, result.MountsCreated)
	require.Zero(t, result.SecretsWritten)
	require.Empty(t, fake.namespaces)
	require.Empty(t, fake.mounts)
	require.Empty(t, fake.writes)
}

func TestEngineRun_EmitsProgress(t *testing.T) {
	fake := &fakeVaultWriter{}
	events := 0

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 2,
		TotalSecrets:   5,
		KV2Probability: 0.9,
		RandomSeed:     123,
		OnProgress: func(snapshot ProgressSnapshot) {
			events++
			require.Equal(t, 5, snapshot.TotalSecrets)
		},
	})

	_, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.GreaterOrEqual(t, events, 1)
}
