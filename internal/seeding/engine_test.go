// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package seeding

import (
	"context"
	"fmt"
	"math/rand"
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
	mountVersions      map[string]int // namespace+":"+mountPath → version
	getMountVersionErr func(namespace, mountPath string) error
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

func (f *fakeVaultWriter) GetMountKVVersions(ctx context.Context, namespace string, mountPaths []string) (map[string]int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	versions := make(map[string]int, len(mountPaths))
	for _, mp := range mountPaths {
		key := namespace + ":" + mp
		if f.getMountVersionErr != nil {
			if err := f.getMountVersionErr(namespace, mp); err != nil {
				return nil, err
			}
		}
		v, ok := f.mountVersions[key]
		if !ok {
			return nil, fmt.Errorf("mount %q not found in namespace %q", mp, namespace)
		}
		versions[mp] = v
	}
	return versions, nil
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
	setupRNG := rand.New(rand.NewSource(123))
	setupPlan, err := Allocate(AllocationInput{
		NamespaceCount: 3,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RNG:            setupRNG,
	})
	require.NoError(t, err)

	mountVersions := make(map[string]int)
	for _, ns := range setupPlan {
		for _, m := range ns.Mounts {
			// No prefix in this test, so withPrefix("", x) == x.
			key := ns.Name + ":" + m.Name
			mountVersions[key] = m.KVVersion
		}
	}
	fake := &fakeVaultWriter{mountVersions: mountVersions}

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

func TestEngineRun_SkipMounts_DiscoversMountVersions(t *testing.T) {
	// Use seed 42 and fixed parameters so we can inspect the exact plan.
	// With KV2Probability=0 every mount will be KV1 in the random plan,
	// but we configure the fake to report KV2 — the engine must use the
	// discovered version, not the planned one.
	const seed int64 = 42
	rng := rand.New(rand.NewSource(seed))
	plan, err := Allocate(AllocationInput{
		NamespaceCount: 2,
		TotalSecrets:   4,
		KV2Probability: 0, // plan will assign KV1 to everything
		RNG:            rng,
	})
	require.NoError(t, err)

	// Build the fake's mountVersions map so every mount reports KV2.
	nsPrefix := ""
	mountPrefix := ""
	mountVersions := make(map[string]int)
	for _, ns := range plan {
		nsName := withPrefix(nsPrefix, ns.Name)
		for _, m := range ns.Mounts {
			mountName := withPrefix(mountPrefix, m.Name)
			key := nsName + ":" + mountName
			mountVersions[key] = 2 // override: actual version is KV2
		}
	}

	fake := &fakeVaultWriter{mountVersions: mountVersions}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 2,
		TotalSecrets:   4,
		KV2Probability: 0,
		RandomSeed:     seed,
		SkipMounts:     true,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Greater(t, result.SecretsWritten, 0)

	// Every write must use version 2 (discovered), not version 1 (planned).
	fake.mu.Lock()
	defer fake.mu.Unlock()
	for _, w := range fake.writes {
		require.Equal(t, 2, w.version,
			"expected write on mount %q in ns %q to use discovered version 2, got %d",
			w.mountPath, w.namespace, w.version)
	}
}

func TestEngineRun_SkipMounts_ErrorWhenMountNotFound(t *testing.T) {
	// The fake has no mountVersions configured, so any lookup returns an error.
	fake := &fakeVaultWriter{}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 1,
		TotalSecrets:   2,
		KV2Probability: 0.5,
		RandomSeed:     7,
		SkipMounts:     true,
	})

	_, err := engine.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "not found")
}

func TestEngineRun_ToleratesFailuresBelowMaxFailures(t *testing.T) {
	called := 0
	fake := &fakeVaultWriter{
		createNamespaceErr: func(namespace string) error {
			called++
			if called <= 2 {
				return fmt.Errorf("transient namespace error")
			}
			return nil
		},
	}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 4,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RandomSeed:     123,
		Workers:        1,
		MaxFailures:    3,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, result.Failures)
	require.Greater(t, result.MountsCreated, 0)
	require.Greater(t, result.SecretsWritten, 0)
}

func TestEngineRun_StopsWhenMaxFailuresReached(t *testing.T) {
	fake := &fakeVaultWriter{
		createNamespaceErr: func(namespace string) error {
			return fmt.Errorf("permanent namespace error")
		},
	}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 5,
		TotalSecrets:   20,
		KV2Probability: 0.9,
		RandomSeed:     123,
		Workers:        1,
		MaxFailures:    2,
	})

	result, err := engine.Run(context.Background())
	require.ErrorIs(t, err, ErrFailureThresholdExceeded)
	require.GreaterOrEqual(t, result.Failures, 2)
	require.Empty(t, fake.mounts)
	require.Empty(t, fake.writes)
}

func TestEngineRun_NeverStopsWithUnlimitedMaxFailures(t *testing.T) {
	fake := &fakeVaultWriter{
		createNamespaceErr: func(namespace string) error {
			return fmt.Errorf("all namespaces fail")
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
		MaxFailures:    -1,
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Equal(t, 3, result.Failures)
	require.Greater(t, result.MountsCreated, 0)
	require.Greater(t, result.SecretsWritten, 0)
}

func TestEngineRun_RejectsInvalidMaxFailures(t *testing.T) {
	fake := &fakeVaultWriter{}
	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 1,
		TotalSecrets:   1,
		KV2Probability: 0.5,
		RandomSeed:     1,
		MaxFailures:    -2,
	})

	_, err := engine.Run(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "MaxFailures")
}

func TestEngineRun_SkipMounts_DiscoveryFailureCountsAgainstBudget(t *testing.T) {
	// The first namespace's mount-version discovery fails. With MaxFailures=-1,
	// the engine records the failure, skips that namespace's secrets, and
	// continues processing the remaining namespaces.
	// Use a seed that allocates exactly 1 mount per namespace.
	rng := rand.New(rand.NewSource(99))
	plan, err := Allocate(AllocationInput{
		NamespaceCount: 2,
		TotalSecrets:   4,
		KV2Probability: 0.5,
		RNG:            rng,
	})
	require.NoError(t, err)

	mountVersions := make(map[string]int)
	for _, ns := range plan {
		for _, mount := range ns.Mounts {
			mountVersions[ns.Name+":"+mount.Name] = mount.KVVersion
		}
	}

	fake := &fakeVaultWriter{
		mountVersions: mountVersions,
		getMountVersionErr: func(namespace, mountPath string) error {
			if namespace == "seed-ns-001" {
				return fmt.Errorf("simulated discovery error")
			}
			return nil
		},
	}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 2,
		TotalSecrets:   4,
		KV2Probability: 0.5,
		RandomSeed:     99,
		Workers:        1,
		SkipMounts:     true,
		MaxFailures:    -1, // never stop; all failures are recorded
	})

	result, err := engine.Run(context.Background())
	require.NoError(t, err)
	require.Greater(t, result.Failures, 0, "discovery failure must be counted")
	require.Greater(t, result.SecretsWritten, 0, "successful namespaces must continue processing")
}

func TestEngineRun_SkipMounts_DiscoveryCancellationPropagatesContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rng := rand.New(rand.NewSource(99))
	plan, err := Allocate(AllocationInput{
		NamespaceCount: 2,
		TotalSecrets:   4,
		KV2Probability: 0.5,
		RNG:            rng,
	})
	require.NoError(t, err)

	mountVersions := make(map[string]int)
	for _, ns := range plan {
		for _, mount := range ns.Mounts {
			mountVersions[ns.Name+":"+mount.Name] = mount.KVVersion
		}
	}

	fake := &fakeVaultWriter{
		mountVersions: mountVersions,
		getMountVersionErr: func(namespace, mountPath string) error {
			cancel()
			return ctx.Err()
		},
	}

	engine := NewEngine(Config{
		Writer:         fake,
		RateLimiter:    noOpLimiter{},
		Retry:          retry.Config{MaxAttempts: 1},
		NamespaceCount: 2,
		TotalSecrets:   4,
		KV2Probability: 0.5,
		RandomSeed:     99,
		Workers:        1,
		SkipNamespaces: true,
		SkipMounts:     true,
	})

	result, err := engine.Run(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.NotErrorIs(t, err, ErrFailureThresholdExceeded)
	require.Zero(t, result.Failures)
}
