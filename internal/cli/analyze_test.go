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

func TestRunAnalyze_WritesAccessData(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")
	outputPath := filepath.Join(dir, "access.pb")

	logData := `{"time":"2024-01-01T00:00:00Z","type":"response","request":{"operation":"read","path":"secret/data/app/config","mount_type":"kv","mount_accessor":"kv_1","namespace":{"id":"ns1","path":"team-a"}}}
{"time":"2024-01-01T00:01:00Z","type":"response","request":{"operation":"read","path":"secret/data/app/config","mount_type":"kv","mount_accessor":"kv_1","namespace":{"id":"ns1","path":"team-a"}}}`
	require.NoError(t, os.WriteFile(logPath, []byte(logData), 0o644))

	err := runAnalyze(context.Background(), analyzeOptions{
		auditLog: logPath,
		output:   outputPath,
	})
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var access vpb.AccessData
	require.NoError(t, gproto.Unmarshal(data, &access))
	require.Len(t, access.Records, 1)
	require.EqualValues(t, 2, access.Records[0].AccessCount)
	require.EqualValues(t, 1, access.Stats.UniqueSecrets)
}

func TestRunAnalyze_NoFilesFound(t *testing.T) {
	err := runAnalyze(context.Background(), analyzeOptions{auditLog: filepath.Join(t.TempDir(), "*.missing")})
	require.Error(t, err)
}
