package correlation

import (
	"testing"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestEngineCorrelate(t *testing.T) {
	engine := NewEngine(Config{MatchingStrategy: MatchingStrategyStrict})
	inventory := &vpb.Inventory{
		Namespaces: []*vpb.Namespace{
			{
				Path: "prod/app",
				Id:   "ns_1",
				Mounts: []*vpb.Mount{
					{
						Path:     "secret/",
						Accessor: "kv_1",
						Secrets: []*vpb.Secret{
							{Path: "app/config"},
							{Path: "app/unused"},
						},
					},
				},
			},
		},
	}
	access := &vpb.AccessData{
		Records: []*vpb.AccessRecord{
			{
				NamespacePath: "prod/app",
				NamespaceId:   "ns_1",
				MountPath:     "secret/",
				MountAccessor: "kv_1",
				SecretPath:    "app/config",
				LastAccessed:  "2025-01-01T00:00:00Z",
			},
		},
	}

	result, err := engine.Correlate(inventory, access)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Contains(t, result, BuildKey(inventory.Namespaces[0], inventory.Namespaces[0].Mounts[0], inventory.Namespaces[0].Mounts[0].Secrets[0]))
}
