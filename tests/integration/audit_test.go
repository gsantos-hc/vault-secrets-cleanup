// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/audit"
	"github.com/stretchr/testify/require"
)

func TestAuditPipelineIntegration(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "audit.log")

	data := `{"time":"2026-04-10T00:00:00Z","type":"response","request":{"operation":"read","path":"secret/data/app/config","mount_type":"kv","mount_accessor":"kv_1","namespace":{"path":"prod/app"}}}
{"time":"2026-04-10T00:05:00Z","type":"response","request":{"operation":"update","path":"secret/data/app/config","mount_type":"kv","mount_accessor":"kv_1","namespace":{"path":"prod/app"}}}`
	require.NoError(t, os.WriteFile(logPath, []byte(data), 0o644))

	reader, err := audit.OpenFile(logPath)
	require.NoError(t, err)
	defer reader.Close()

	parser := audit.NewParser()
	extractor := audit.NewExtractor()
	aggregator := audit.NewAggregator()

	err = parser.Parse(context.Background(), reader, func(event audit.AuditEvent) error {
		aggregator.Add(extractor.Extract(event))
		return nil
	})
	require.NoError(t, err)

	records := aggregator.GetRecords()
	require.Len(t, records, 1)
	require.Equal(t, "prod/app", records[0].NamespacePath)
	require.Equal(t, "secret/", records[0].MountPath)
	require.Equal(t, "app/config", records[0].SecretPath)
	require.Equal(t, int32(2), records[0].AccessCount)
	require.Equal(t, "write", records[0].AccessType)

	last, parseErr := time.Parse(time.RFC3339, records[0].LastAccessed)
	require.NoError(t, parseErr)
	require.Equal(t, "2026-04-10T00:05:00Z", last.UTC().Format(time.RFC3339))
}
