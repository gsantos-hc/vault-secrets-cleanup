// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"
)

func TestFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping e2e test in short mode")
	}

	repoRoot, err := os.Getwd()
	require.NoError(t, err)
	repoRoot = filepath.Clean(filepath.Join(repoRoot, "..", ".."))

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	auditLog := filepath.Join(dir, "audit.log")
	inventoryPath := filepath.Join(dir, "inventory.pb")
	accessPath := filepath.Join(dir, "access.pb")
	planPath := filepath.Join(dir, "deletion-plan.pb")
	reportPath := filepath.Join(dir, "report.json")

	cfg := "vault:\n  address: http://127.0.0.1:8200\n  token: root\n"
	require.NoError(t, os.WriteFile(configPath, []byte(cfg), 0o644))

	logData := fmt.Sprintf(
		`{"time":%q,"type":"response","request":{"operation":"read","path":"secret/data/app/config","mount_type":"kv","mount_accessor":"kv_1","namespace":{"path":"prod/app"}}}\n`,
		time.Now().UTC().AddDate(-2, 0, 0).Format(time.RFC3339),
	)
	require.NoError(t, os.WriteFile(auditLog, []byte(logData), 0o644))

	inventory := &vpb.Inventory{Namespaces: []*vpb.Namespace{{
		Path: "prod/app",
		Mounts: []*vpb.Mount{{
			Path:     "secret/",
			Accessor: "kv_1",
			Version:  2,
			Secrets:  []*vpb.Secret{{Path: "app/config"}},
		}},
	}}}
	inventoryBytes, err := gproto.Marshal(inventory)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(inventoryPath, inventoryBytes, 0o644))

	t.Run("analyze", func(t *testing.T) {
		runCLI(t, repoRoot,
			"--config", configPath,
			"analyze",
			"--audit-log", auditLog,
			"--output", accessPath,
		)
		require.FileExists(t, accessPath)
	})

	t.Run("plan", func(t *testing.T) {
		runCLI(t, repoRoot,
			"--config", configPath,
			"plan",
			"--inventory", inventoryPath,
			"--access-data", accessPath,
			"--staleness-period", "365d",
			"--output", planPath,
		)
		require.FileExists(t, planPath)
	})

	t.Run("execute_dry_run", func(t *testing.T) {
		runCLI(t, repoRoot,
			"--config", configPath,
			"execute",
			"--plan", planPath,
			"--dry-run",
			"--yes",
		)
	})

	t.Run("report", func(t *testing.T) {
		runCLI(t, repoRoot,
			"--config", configPath,
			"report",
			"--plan", planPath,
			"--format", "json",
			"--output", reportPath,
		)
		require.FileExists(t, reportPath)
	})
}

func runCLI(t *testing.T, repoRoot string, args ...string) {
	t.Helper()
	cmdArgs := append([]string{"run", "./cmd/vault-secrets-cleanup"}, args...)
	cmd := exec.CommandContext(context.Background(), "go", cmdArgs...)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "command failed: go %v\noutput:\n%s", cmdArgs, string(out))
}
