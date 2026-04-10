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

func TestRunImport_WritesAccessData(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "records.json")
	output := filepath.Join(dir, "access.pb")

	jsonData := `[
  {
    "namespace_path": "team-a",
    "mount_path": "secret/",
    "mount_accessor": "kv_1",
    "secret_path": "app/config",
    "last_accessed": "2024-01-01T00:00:00Z",
    "access_type": "read",
    "access_count": 3
  }
]`
	require.NoError(t, os.WriteFile(input, []byte(jsonData), 0o644))

	err := runImport(context.Background(), importOptions{input: input, output: output})
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)

	var access vpb.AccessData
	require.NoError(t, gproto.Unmarshal(data, &access))
	require.Len(t, access.Records, 1)
	require.EqualValues(t, 3, access.Records[0].AccessCount)
	require.EqualValues(t, 1, access.Stats.TotalRecords)
}

func TestRunImport_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "records.json")
	require.NoError(t, os.WriteFile(input, []byte("{"), 0o644))

	err := runImport(context.Background(), importOptions{input: input, output: filepath.Join(dir, "out.pb")})
	require.Error(t, err)
}
