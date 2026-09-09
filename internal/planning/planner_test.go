// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package planning

import (
	"testing"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestPlannerGeneratePlan(t *testing.T) {
	planner := NewPlanner(Config{
		StalenessConfig: StalenessConfig{DefaultPeriod: "30d"},
		ExclusionConfig: ExclusionConfig{PathPatterns: []string{"secret/ignore/*"}},
		IncludeUnknown:  false,
	})

	inv := &vpb.Inventory{Namespaces: []*vpb.Namespace{{
		Path: "prod/app",
		Mounts: []*vpb.Mount{{
			Path:     "secret/",
			Accessor: "kv_1",
			Secrets: []*vpb.Secret{
				{Path: "old/config"},
				{Path: "ignore/special"},
				{Path: "unknown/path"},
			},
		}},
	}}}

	staleAccess := &vpb.AccessRecord{LastAccessed: time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339)}
	accessMap := map[string]*vpb.AccessRecord{
		"prod/app|kv_1|old/config": staleAccess,
	}

	plan, err := planner.GeneratePlan(inv, accessMap, &vpb.PlanConfig{StalenessPeriod: "30d"})
	require.NoError(t, err)
	require.NotNil(t, plan.Stats)
	require.Len(t, plan.Actions, 3)
	require.EqualValues(t, 1, plan.Stats.StaleCount)
	require.EqualValues(t, 1, plan.Stats.UnknownCount)
	require.EqualValues(t, 1, plan.Stats.ExcludedCount)
	require.EqualValues(t, 1, plan.Stats.ToDeleteCount)
}
