package reporting

import (
	"testing"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestGeneratorGenerate(t *testing.T) {
	g := NewGenerator()
	plan := &vpb.DeletionPlan{
		Stats: &vpb.PlanStats{TotalSecrets: 2, StaleCount: 1, ToDeleteCount: 1},
		Actions: []*vpb.SecretAction{
			{NamespacePath: "prod/app", MountPath: "secret/", SecretPath: "old/config", Category: "stale", LastAccessed: time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339), Reason: "old"},
			{NamespacePath: "prod/app", MountPath: "secret/", SecretPath: "new/config", Category: "active", Reason: "recent"},
		},
	}

	out, err := g.Generate(plan, "markdown")
	require.NoError(t, err)
	require.Contains(t, out, "# Deletion Plan Report")
	require.Contains(t, out, "Last Access")
	require.Contains(t, out, "10m ago")

	out, err = g.Generate(plan, "json")
	require.NoError(t, err)
	require.Contains(t, out, "\"total_secrets\":2")

	out, err = g.Generate(plan, "csv")
	require.NoError(t, err)
	require.Contains(t, out, "namespace_path,mount_path,secret_path,category,last_access_age,reason")
	require.Contains(t, out, "10m ago")

	_, err = g.Generate(plan, "xml")
	require.Error(t, err)
}
