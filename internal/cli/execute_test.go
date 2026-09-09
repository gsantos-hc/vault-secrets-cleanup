// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/deletion"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"
)

type fakeDeletionEngine struct {
	run func(plan *vpb.DeletionPlan) error
}

func (f fakeDeletionEngine) Execute(_ context.Context, plan *vpb.DeletionPlan) error {
	if f.run != nil {
		return f.run(plan)
	}
	return nil
}

func TestRunExecute_CancelledByConfirmation(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.pb")
	require.NoError(t, writeTestPlan(planPath))

	originalCreateEngine := createExecuteEngine
	originalCreateClient := createExecuteVaultClient
	defer func() {
		createExecuteEngine = originalCreateEngine
		createExecuteVaultClient = originalCreateClient
	}()

	createExecuteVaultClient = func(_ *config.Config) (executeVaultClient, error) {
		return nil, nil
	}
	createExecuteEngine = func(_ executeVaultClient, _ executeOptions, _ *config.Config, _ func(deletion.ProgressSnapshot)) deletionEngine {
		t.Fatal("engine should not be created when confirmation is declined")
		return nil
	}

	out := &bytes.Buffer{}
	err := runExecute(context.Background(), executeOptions{plan: planPath, confirmAll: false}, &config.Config{}, bytes.NewBufferString("no\n"), out)
	require.NoError(t, err)
	require.Contains(t, out.String(), "Deletion cancelled")
}

func TestRunExecute_DryRunSkipsConfirmationAndRunsEngine(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.pb")
	require.NoError(t, writeTestPlan(planPath))

	originalCreateEngine := createExecuteEngine
	originalCreateClient := createExecuteVaultClient
	defer func() {
		createExecuteEngine = originalCreateEngine
		createExecuteVaultClient = originalCreateClient
	}()

	called := false
	createExecuteVaultClient = func(_ *config.Config) (executeVaultClient, error) {
		return nil, nil
	}
	createExecuteEngine = func(_ executeVaultClient, _ executeOptions, _ *config.Config, _ func(deletion.ProgressSnapshot)) deletionEngine {
		return fakeDeletionEngine{run: func(plan *vpb.DeletionPlan) error {
			called = true
			require.Len(t, plan.Actions, 1)
			return nil
		}}
	}

	out := &bytes.Buffer{}
	err := runExecute(context.Background(), executeOptions{plan: planPath, dryRun: true}, &config.Config{}, bytes.NewBufferString(""), out)
	require.NoError(t, err)
	require.True(t, called)
}

func TestResolveParallelWorkers_DefaultFromConfig(t *testing.T) {
	workers := resolveParallelWorkers(executeOptions{}, &config.Config{
		Parallel: config.ParallelConfig{Workers: 12},
	})
	require.Equal(t, 12, workers)
}

func TestResolveParallelWorkers_FlagOverridesConfig(t *testing.T) {
	workers := resolveParallelWorkers(executeOptions{workers: 4}, &config.Config{
		Parallel: config.ParallelConfig{Workers: 12},
	})
	require.Equal(t, 4, workers)
}

func TestResolveParallelWorkers_DefaultWhenUnset(t *testing.T) {
	workers := resolveParallelWorkers(executeOptions{}, &config.Config{})
	require.Equal(t, 10, workers)
}

func writeTestPlan(path string) error {
	plan := &vpb.DeletionPlan{
		Stats: &vpb.PlanStats{TotalSecrets: 1, ToDeleteCount: 1, StaleCount: 1},
		Actions: []*vpb.SecretAction{{
			NamespacePath: "ns",
			MountPath:     "secret/",
			SecretPath:    "app/key",
			Category:      "stale",
			Status:        "pending",
		}},
	}

	blob, err := gproto.Marshal(plan)
	if err != nil {
		return err
	}
	return os.WriteFile(path, blob, 0o644)
}
