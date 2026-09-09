// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

//go:build integration

package integration

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/deletion"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

type dryRunDeleter struct{}

func (d dryRunDeleter) KVVersion(_ context.Context, _ string, _ string) (int, error) {
	return 2, nil
}

func (d dryRunDeleter) DeleteKVv1(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (d dryRunDeleter) DeleteKVv2(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func TestDeletionEngineDryRunIntegration(t *testing.T) {
	plan := &vpb.DeletionPlan{
		Config: &vpb.PlanConfig{IncludeUnknown: false},
		Actions: []*vpb.SecretAction{{
			NamespacePath: "root",
			MountPath:     "secret/",
			SecretPath:    "app/key",
			Category:      "stale",
			Status:        "pending",
		}},
	}

	eng := deletion.NewEngine(deletion.Config{
		Deleter:        dryRunDeleter{},
		RateLimiter:    ratelimit.New(100),
		RetryConfig:    retry.DefaultConfig(),
		CircuitBreaker: deletion.NewCircuitBreaker(3),
		DryRun:         true,
		PlanFile:       filepath.Join(t.TempDir(), "plan.pb"),
	})

	err := eng.Execute(context.Background(), plan)
	require.NoError(t, err)
	require.Equal(t, "pending", plan.Actions[0].Status)
}
