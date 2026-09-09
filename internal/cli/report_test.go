// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
	gproto "google.golang.org/protobuf/proto"
)

func TestRunReport_WritesMarkdown(t *testing.T) {
	dir := t.TempDir()
	planPath := filepath.Join(dir, "plan.pb")
	outputPath := filepath.Join(dir, "report.md")

	plan := &vpb.DeletionPlan{
		Stats: &vpb.PlanStats{TotalSecrets: 1, StaleCount: 1, ToDeleteCount: 1},
		Actions: []*vpb.SecretAction{{
			NamespacePath: "prod/app",
			MountPath:     "secret/",
			SecretPath:    "old/config",
			Category:      "stale",
			Reason:        "old",
		}},
	}
	bytes, err := gproto.Marshal(plan)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(planPath, bytes, 0o644))

	err = runReport(context.Background(), reportOptions{planPath: planPath, format: "markdown", output: outputPath})
	require.NoError(t, err)

	reportBytes, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	require.Contains(t, string(reportBytes), "# Deletion Plan Report")
}
