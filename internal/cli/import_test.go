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

func TestRunImport_ExtractorJSONArray_NormalizesAndAggregatesKVv2Paths(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "extract.json")
	output := filepath.Join(dir, "access.pb")

	jsonData := `[
{"namespace_path":"team-a/","mount_path":"team-a/secret/","secret_path":"secret/subkeys/app/config","timestamp":"2024-01-01T00:00:00Z"},
{"namespace_path":"team-a/","mount_path":"team-a/secret/","secret_path":"secret/metadata/app/config","timestamp":"2024-01-02T00:00:00Z"},
{"namespace_path":"team-a/","mount_path":"team-a/secret/","secret_path":"secret/data/app/config","timestamp":"2024-01-03T00:00:00Z"}
]`
	require.NoError(t, os.WriteFile(input, []byte(jsonData), 0o644))

	err := runImport(context.Background(), importOptions{input: input, output: output})
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)

	var access vpb.AccessData
	require.NoError(t, gproto.Unmarshal(data, &access))
	require.Len(t, access.Records, 1)

	record := access.Records[0]
	require.Equal(t, "team-a/", record.NamespacePath)
	require.Equal(t, "secret/", record.MountPath)
	require.Equal(t, "app/config", record.SecretPath)
	require.Equal(t, "2024-01-03T00:00:00Z", record.LastAccessed)
	require.Equal(t, "read", record.AccessType)
	require.EqualValues(t, 3, record.AccessCount)
	require.EqualValues(t, 1, access.Stats.TotalRecords)
}

func TestRunImport_ExtractorJSONArray_MergesDifferentMountPathsWithSameAccessorAndSecret(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "extract.json")
	output := filepath.Join(dir, "access.pb")

	jsonData := `[
{"namespace_path":"team-a/","mount_accessor":"kv_shared","mount_path":"team-a/secret/","secret_path":"secret/data/app/config","timestamp":"2024-01-01T00:00:00Z"},
{"namespace_path":"team-a/","mount_accessor":"kv_shared","mount_path":"team-a/kv2/","secret_path":"kv2/data/app/config","timestamp":"2024-01-02T00:00:00Z"}
]`
	require.NoError(t, os.WriteFile(input, []byte(jsonData), 0o644))

	err := runImport(context.Background(), importOptions{input: input, output: output})
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)

	var access vpb.AccessData
	require.NoError(t, gproto.Unmarshal(data, &access))
	require.Len(t, access.Records, 1)

	require.EqualValues(t, 1, access.Stats.TotalRecords)
	require.EqualValues(t, 2, access.Records[0].AccessCount)
	require.Equal(t, "kv_shared", access.Records[0].MountAccessor)
	require.Equal(t, "app/config", access.Records[0].SecretPath)
}

func TestRunImport_ExtractorJSONArray_DoesNotMergeDifferentMountAccessors(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "extract.json")
	output := filepath.Join(dir, "access.pb")

	jsonData := `[
{"namespace_path":"team-a/","mount_accessor":"kv_a","mount_path":"team-a/secret/","secret_path":"secret/data/app/config","timestamp":"2024-01-01T00:00:00Z"},
{"namespace_path":"team-a/","mount_accessor":"kv_b","mount_path":"team-a/secret/","secret_path":"secret/metadata/app/config","timestamp":"2024-01-02T00:00:00Z"}
]`
	require.NoError(t, os.WriteFile(input, []byte(jsonData), 0o644))

	err := runImport(context.Background(), importOptions{input: input, output: output})
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)

	var access vpb.AccessData
	require.NoError(t, gproto.Unmarshal(data, &access))
	require.Len(t, access.Records, 2)
	require.EqualValues(t, 2, access.Stats.TotalRecords)
}

func TestRunImport_ExtractorJSONArray_MergesSameAccessorAndSecretAcrossNamespacesAndMounts(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "extract.json")
	output := filepath.Join(dir, "access.pb")

	jsonData := `[
{"namespace_path":"team-a/","mount_accessor":"kv_shared","mount_path":"team-a/secret/","secret_path":"secret/data/app/config","timestamp":"2024-01-01T00:00:00Z"},
{"namespace_path":"team-b/","mount_accessor":"kv_shared","mount_path":"team-b/moved-secret/","secret_path":"moved-secret/metadata/app/config","timestamp":"2024-01-03T00:00:00Z"}
]`
	require.NoError(t, os.WriteFile(input, []byte(jsonData), 0o644))

	err := runImport(context.Background(), importOptions{input: input, output: output})
	require.NoError(t, err)

	data, err := os.ReadFile(output)
	require.NoError(t, err)

	var access vpb.AccessData
	require.NoError(t, gproto.Unmarshal(data, &access))
	require.Len(t, access.Records, 1)
	require.EqualValues(t, 1, access.Stats.TotalRecords)

	record := access.Records[0]
	require.Equal(t, "kv_shared", record.MountAccessor)
	require.Equal(t, "app/config", record.SecretPath)
	require.EqualValues(t, 2, record.AccessCount)
	require.Equal(t, "2024-01-03T00:00:00Z", record.LastAccessed)
}
