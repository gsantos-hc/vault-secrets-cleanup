package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"
)

func TestRunPlan_WritesDeletionPlan(t *testing.T) {
	dir := t.TempDir()
	inventoryPath := filepath.Join(dir, "inventory.pb")
	accessPath := filepath.Join(dir, "access.pb")
	outputPath := filepath.Join(dir, "plan.pb")

	inventory := &vpb.Inventory{Namespaces: []*vpb.Namespace{{
		Path: "prod/app",
		Id:   "ns_1",
		Mounts: []*vpb.Mount{{
			Path:     "secret/",
			Accessor: "kv_1",
			Secrets:  []*vpb.Secret{{Path: "old/config"}},
		}},
	}}}
	access := &vpb.AccessData{Records: []*vpb.AccessRecord{{
		NamespacePath: "prod/app",
		NamespaceId:   "ns_1",
		MountPath:     "secret/",
		MountAccessor: "kv_1",
		SecretPath:    "old/config",
		LastAccessed:  time.Now().UTC().AddDate(0, 0, -90).Format(time.RFC3339),
	}}}

	invBytes, err := gproto.Marshal(inventory)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(inventoryPath, invBytes, 0o644))

	accBytes, err := gproto.Marshal(access)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(accessPath, accBytes, 0o644))

	err = runPlan(context.Background(), planOptions{
		inventory:        inventoryPath,
		accessData:       accessPath,
		output:           outputPath,
		stalenessPeriod:  "30d",
		matchingStrategy: "strict",
		includeUnknown:   false,
	}, &config.Config{Staleness: config.StalenessConfig{DefaultPeriod: "365d"}})
	require.NoError(t, err)

	plBytes, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var plan vpb.DeletionPlan
	require.NoError(t, gproto.Unmarshal(plBytes, &plan))
	require.EqualValues(t, 1, plan.Stats.StaleCount)
	require.EqualValues(t, 1, plan.Stats.ToDeleteCount)
	require.Equal(t, "30d", plan.Config.StalenessPeriod)
	require.Equal(t, inventoryPath, plan.InventoryFile)
	require.Equal(t, accessPath, plan.AccessFile)
}
