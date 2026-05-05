// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package seeding

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	"github.com/stretchr/testify/require"
)

type fakeVaultWriter struct {
	mu                 sync.Mutex
	namespaces         []string
	mounts             []mountCall
	writes             []writeCall
	onCreateNamespace  func(namespace string)
	onEnableKVMount    func(namespace, mountPath string)
	onWriteKVSecret    func(namespace, mountPath, secretPath string)
	createNamespaceErr func(namespace string) error
	enableMountErr     func(namespace, mountPath string) error
	writeSecretErr     func(namespace, mountPath, secretPath string) error
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
	if f.onCreateNamespace != nil {
		f.onCreateNamespace(namespace)
	}
	if f.createNamespaceErr != nil {
		if err := f.createNamespaceErr(namespace); err != nil {
			return err
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.namespaces = append(f.namespaces, namespace)
	return nil
}

func (f *fakeVaultWriter) EnableKVMount(ctx context.Context, namespace, mountPath string, kvVersion int) error {
	if f.onEnableKVMount != nil {
		f.onEnableKVMount(namespace, mountPath)
	}
	if f.enableMountErr != nil {
		if err := f.enableMountErr(namespace, mountPath); err != nil {
			return err
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mounts = append(f.mounts, mountCall{namespace: namespace, path: mountPath, version: kvVersion})
	return nil
}

func (f *fakeVaultWriter) WriteKVSecret(ctx context.Context, namespace, mountPath, secretPath string, data map[string]any, kvVersion int) error {
	if f.onWriteKVSecret != nil {
		f.onWriteKVSecret(namespace, mountPath, secretPath)
	}
	if f.writeSecretErr != nil {
		if err := f.writeSecretErr(namespace, mountPath, secretPath); err != nil {
			return err
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writes = append(f.writes, writeCall{namespace: namespace, mountPath: mountPath, path: secretPath, version: kvVersion})
	return nil
}

type noOpLimiter struct{}

func (n noOpLimiter) Wait(ctx context.Context) error {
	return nil
}

type concurrencyProbe struct {
	mu      sync.Mutex
	current int
	max     int
}

func (p *concurrencyProbe) enter() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current++
	if p.current > p.max {
		p.max = p.current
	}
}

func (p *concurrencyProbe) leave() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current--
}

func (p *concurrencyProbe) maxSeen() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.max
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

func TestEngineRun_CreatesAllNamespacesBeforeEnablingAnyMount(t *testing.T) {
	var (
		mu                sync.Mutex
		namespaceCalls    int
		firstMountAtCount int
	)

	fake := &fakeVaultWriter{
		onCreateNamespace: func(namespace string) {
			mu.Lock()
			defer mu.Unlock()
			namespaceCalls++
		},
		onEnableKVMount: func(namespace, mountPath string) {
			mu.Lock()
			defer mu.Unlock()
			if firstMountAtCount == 0 {
				firstMountAtCount = namespaceCalls
			}
		},
	}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 3,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RandomSeed:     123,
	})

	_, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, firstMountAtCount, "expected mount creation to start only after all namespaces were created")
}

func TestEngineRun_UsesWorkersAcrossAllStages(t *testing.T) {
	var namespaceProbe concurrencyProbe
	var mountProbe concurrencyProbe
	var secretProbe concurrencyProbe

	fake := &fakeVaultWriter{
		onCreateNamespace: func(namespace string) {
			namespaceProbe.enter()
			defer namespaceProbe.leave()
			time.Sleep(10 * time.Millisecond)
		},
		onEnableKVMount: func(namespace, mountPath string) {
			mountProbe.enter()
			defer mountProbe.leave()
			time.Sleep(10 * time.Millisecond)
		},
		onWriteKVSecret: func(namespace, mountPath, secretPath string) {
			secretProbe.enter()
			defer secretProbe.leave()
			time.Sleep(10 * time.Millisecond)
		},
	}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 4,
		TotalSecrets:   40,
		KV2Probability: 0.9,
		RandomSeed:     123,
		Workers:        4,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Greater(t, result.PlannedMounts, 1)
	require.Greater(t, result.PlannedSecrets, 1)
	require.Greater(t, namespaceProbe.maxSeen(), 1, "expected namespace creation to overlap when workers > 1")
	require.Greater(t, mountProbe.maxSeen(), 1, "expected mount enablement to overlap when workers > 1")
	require.Greater(t, secretProbe.maxSeen(), 1, "expected secret writes to overlap when workers > 1")
}

func TestEngineRun_StopsBeforeLaterStagesWhenNamespaceCreationFails(t *testing.T) {
	fake := &fakeVaultWriter{
		createNamespaceErr: func(namespace string) error {
			if namespace == "seed-ns-002" {
				return context.DeadlineExceeded
			}
			return nil
		},
	}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 3,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RandomSeed:     123,
		Workers:        3,
	})

	result, err := engine.Run(context.Background())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Zero(t, result.MountsCreated)
	require.Zero(t, result.SecretsWritten)
	require.Empty(t, fake.mounts)
	require.Empty(t, fake.writes)
}

func TestEngineRun_SkipNamespaceCreation(t *testing.T) {
	fake := &fakeVaultWriter{}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 3,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RandomSeed:     123,
		SkipNamespaces: true,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.NamespacesCreated)
	require.Empty(t, fake.namespaces)
	require.Greater(t, result.MountsCreated, 0)
	require.Greater(t, result.SecretsWritten, 0)
}

func TestEngineRun_SkipMountCreation(t *testing.T) {
	fake := &fakeVaultWriter{}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 3,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RandomSeed:     123,
		SkipMounts:     true,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Zero(t, result.MountsCreated)
	require.Empty(t, fake.mounts)
	require.Equal(t, 3, result.NamespacesCreated)
	require.Greater(t, result.SecretsWritten, 0)
}
