// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

//go:build integration

package integration

import (
	"testing"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/correlation"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/planning"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestCorrelationAndPlanningIntegration(t *testing.T) {
	inventory := &vpb.Inventory{
		Namespaces: []*vpb.Namespace{{
			Path: "prod/app",
			Mounts: []*vpb.Mount{{
				Path:     "secret/",
				Accessor: "kv_1",
				Version:  2,
				Secrets: []*vpb.Secret{
					{Path: "stale/config"},
					{Path: "active/config"},
				},
			}},
		}},
	}

	accessData := &vpb.AccessData{
		Records: []*vpb.AccessRecord{
			{
				NamespacePath: "prod/app",
				MountPath:     "secret/",
				MountAccessor: "kv_1",
				SecretPath:    "stale/config",
				LastAccessed:  time.Now().UTC().AddDate(-2, 0, 0).Format(time.RFC3339),
				AccessType:    "read",
				AccessCount:   1,
			},
			{
				NamespacePath: "prod/app",
				MountPath:     "secret/",
				MountAccessor: "kv_1",
				SecretPath:    "active/config",
				LastAccessed:  time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339),
				AccessType:    "read",
				AccessCount:   1,
			},
		},
	}

	engine := correlation.NewEngine(correlation.Config{MatchingStrategy: correlation.MatchingStrategyAccessor})
	accessMap, err := engine.Correlate(inventory, accessData)
	require.NoError(t, err)

	planner := planning.NewPlanner(planning.Config{
		StalenessConfig: planning.StalenessConfig{DefaultPeriod: "365d"},
		ExclusionConfig: planning.ExclusionConfig{},
		IncludeUnknown:  false,
	})

	plan, err := planner.GeneratePlan(inventory, accessMap, &vpb.PlanConfig{StalenessPeriod: "365d"})
	require.NoError(t, err)
	require.NotNil(t, plan.Stats)
	require.EqualValues(t, 2, plan.Stats.TotalSecrets)
	require.EqualValues(t, 1, plan.Stats.StaleCount)
	require.EqualValues(t, 1, plan.Stats.ActiveCount)
	require.EqualValues(t, 1, plan.Stats.ToDeleteCount)
}
