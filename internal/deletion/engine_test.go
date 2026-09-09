// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package deletion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"
)

type fakeExecutor struct {
	errs map[string]error
}

func (f *fakeExecutor) Delete(_ context.Context, action *vpb.SecretAction) error {
	if err, ok := f.errs[action.SecretPath]; ok {
		return err
	}
	return nil
}

func TestEngine_Execute_UpdatesStatusesAndWritesPlan(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.pb")
	plan := testPlan(true)

	eng := NewEngine(Config{
		Executor:       &fakeExecutor{},
		RateLimiter:    ratelimit.New(1000),
		RetryConfig:    retry.DefaultConfig(),
		CircuitBreaker: NewCircuitBreaker(2),
		Workers:        1,
		PlanFile:       planPath,
	})

	err := eng.Execute(context.Background(), plan)
	require.NoError(t, err)

	require.Equal(t, "deleted", plan.Actions[0].Status)
	require.Equal(t, "pending", plan.Actions[2].Status)

	data, err := os.ReadFile(planPath)
	require.NoError(t, err)
	var reloaded vpb.DeletionPlan
	require.NoError(t, gproto.Unmarshal(data, &reloaded))
	require.Equal(t, "deleted", reloaded.Actions[0].Status)
}

func TestEngine_Execute_ResumesBySkippingDeleted(t *testing.T) {
	plan := testPlan(false)
	plan.Actions[0].Status = "deleted"

	exec := &countingExecutor{}
	eng := NewEngine(Config{
		Executor:       exec,
		RateLimiter:    ratelimit.New(1000),
		CircuitBreaker: NewCircuitBreaker(2),
		Workers:        1,
		PlanFile:       filepath.Join(t.TempDir(), "plan.pb"),
	})

	err := eng.Execute(context.Background(), plan)
	require.NoError(t, err)
	require.Equal(t, 0, exec.calls)
}

func TestEngine_Execute_StopsWhenCircuitBreakerOpens(t *testing.T) {
	plan := testPlan(false)
	plan.Actions = append(plan.Actions, &vpb.SecretAction{Category: "stale", Status: "pending", SecretPath: "s2"})
	plan.Actions = append(plan.Actions, &vpb.SecretAction{Category: "stale", Status: "pending", SecretPath: "s3"})

	exec := &fakeExecutor{errs: map[string]error{"s1": errors.New("boom"), "s2": errors.New("boom")}}
	eng := NewEngine(Config{
		Executor:       exec,
		RateLimiter:    ratelimit.New(1000),
		CircuitBreaker: NewCircuitBreaker(2),
		Workers:        1,
		PlanFile:       filepath.Join(t.TempDir(), "plan.pb"),
	})

	err := eng.Execute(context.Background(), plan)
	require.Error(t, err)
	require.Equal(t, "failed", plan.Actions[0].Status)
	require.Equal(t, "failed", plan.Actions[3].Status)
	require.Equal(t, "pending", plan.Actions[4].Status)
}

func TestEngine_Execute_UnknownHandledByPlanConfig(t *testing.T) {
	plan := testPlan(false)
	exec := &countingExecutor{}
	eng := NewEngine(Config{
		Executor:       exec,
		RateLimiter:    ratelimit.New(1000),
		CircuitBreaker: NewCircuitBreaker(2),
		Workers:        1,
		PlanFile:       filepath.Join(t.TempDir(), "plan.pb"),
	})

	err := eng.Execute(context.Background(), plan)
	require.NoError(t, err)
	require.Equal(t, 1, exec.calls)
}

func TestEngine_Execute_EmitsProgress(t *testing.T) {
	plan := testPlan(false)
	exec := &countingExecutor{}
	events := 0

	eng := NewEngine(Config{
		Executor:       exec,
		RateLimiter:    ratelimit.New(1000),
		CircuitBreaker: NewCircuitBreaker(2),
		Workers:        1,
		PlanFile:       filepath.Join(t.TempDir(), "plan.pb"),
		OnProgress: func(snapshot ProgressSnapshot) {
			events++
			require.Equal(t, 1, snapshot.Total)
		},
	})

	err := eng.Execute(context.Background(), plan)
	require.NoError(t, err)
	require.GreaterOrEqual(t, events, 1)
}

func TestEngine_Execute_ProcessesInParallel(t *testing.T) {
	plan := &vpb.DeletionPlan{
		Config: &vpb.PlanConfig{IncludeUnknown: false},
		Actions: []*vpb.SecretAction{
			{Category: "stale", Status: "pending", SecretPath: "s1"},
			{Category: "stale", Status: "pending", SecretPath: "s2"},
		},
	}

	exec := newRendezvousExecutor(2)
	eng := NewEngine(Config{
		Executor:       exec,
		RateLimiter:    ratelimit.New(1000),
		CircuitBreaker: NewCircuitBreaker(2),
		PlanFile:       filepath.Join(t.TempDir(), "plan.pb"),
		Workers:        2,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := eng.Execute(ctx, plan)
	require.NoError(t, err)
}

func TestEngine_Execute_PersistsCanceledInFlightAsFailed(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.pb")
	plan := &vpb.DeletionPlan{
		Config: &vpb.PlanConfig{IncludeUnknown: false},
		Actions: []*vpb.SecretAction{
			{Category: "stale", Status: "pending", SecretPath: "s1"},
			{Category: "stale", Status: "pending", SecretPath: "s2"},
			{Category: "stale", Status: "pending", SecretPath: "s3"},
		},
	}

	secondStarted := make(chan struct{}, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	eng := NewEngine(Config{
		Executor:       &interruptingExecutor{secondStarted: secondStarted},
		RateLimiter:    ratelimit.New(1000),
		CircuitBreaker: NewCircuitBreaker(3),
		Workers:        1,
		PlanFile:       planPath,
	})

	go func() {
		select {
		case <-secondStarted:
			cancel()
		case <-ctx.Done():
		}
	}()

	err := eng.Execute(ctx, plan)
	require.ErrorIs(t, err, context.Canceled)

	require.Equal(t, "deleted", plan.Actions[0].Status)
	require.Equal(t, "failed", plan.Actions[1].Status)
	require.Equal(t, context.Canceled.Error(), plan.Actions[1].Error)
	require.Equal(t, "pending", plan.Actions[2].Status)

	data, readErr := os.ReadFile(planPath)
	require.NoError(t, readErr)

	var reloaded vpb.DeletionPlan
	require.NoError(t, gproto.Unmarshal(data, &reloaded))
	require.Equal(t, "deleted", reloaded.Actions[0].Status)
	require.Equal(t, "failed", reloaded.Actions[1].Status)
	require.Equal(t, context.Canceled.Error(), reloaded.Actions[1].Error)
	require.Equal(t, "pending", reloaded.Actions[2].Status)
}

type countingExecutor struct {
	calls int
}

func (c *countingExecutor) Delete(_ context.Context, _ *vpb.SecretAction) error {
	c.calls++
	return nil
}

// rendezvousExecutor blocks each Delete call until n concurrent calls have started,
// confirming that the engine dispatches multiple deletions simultaneously.
type rendezvousExecutor struct {
	mu      sync.Mutex
	once    sync.Once
	n       int
	count   int
	arrived chan struct{}
}

func newRendezvousExecutor(n int) *rendezvousExecutor {
	return &rendezvousExecutor{n: n, arrived: make(chan struct{})}
}

func (r *rendezvousExecutor) Delete(ctx context.Context, _ *vpb.SecretAction) error {
	r.mu.Lock()
	r.count++
	if r.count >= r.n {
		r.once.Do(func() { close(r.arrived) })
	}
	r.mu.Unlock()
	select {
	case <-r.arrived:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type interruptingExecutor struct {
	mu            sync.Mutex
	calls         int
	secondStarted chan struct{}
}

func (i *interruptingExecutor) Delete(ctx context.Context, _ *vpb.SecretAction) error {
	i.mu.Lock()
	i.calls++
	call := i.calls
	i.mu.Unlock()

	if call == 1 {
		return nil
	}
	if call == 2 {
		select {
		case i.secondStarted <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	}

	return nil
}

func testPlan(includeUnknown bool) *vpb.DeletionPlan {
	return &vpb.DeletionPlan{
		Config: &vpb.PlanConfig{IncludeUnknown: includeUnknown},
		Actions: []*vpb.SecretAction{
			{Category: "stale", Status: "pending", SecretPath: "s1", NamespacePath: "ns", MountPath: "secret/"},
			{Category: "unknown", Status: "pending", SecretPath: "u1", NamespacePath: "ns", MountPath: "secret/"},
			{Category: "active", Status: "pending", SecretPath: "a1", NamespacePath: "ns", MountPath: "secret/"},
		},
	}
}
