package deletion

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
		PlanFile:       filepath.Join(t.TempDir(), "plan.pb"),
	})

	err := eng.Execute(context.Background(), plan)
	require.NoError(t, err)
	require.Equal(t, 1, exec.calls)
}

type countingExecutor struct {
	calls int
}

func (c *countingExecutor) Delete(_ context.Context, _ *vpb.SecretAction) error {
	c.calls++
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
