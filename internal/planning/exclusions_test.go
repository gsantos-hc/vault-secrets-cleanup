package planning

import (
	"testing"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestExclusionEngineIsExcluded(t *testing.T) {
	engine := NewExclusionEngine(ExclusionConfig{
		NamespacePatterns: []string{"admin/*"},
		PathPatterns:      []string{"secret/bootstrap/*"},
		MountPatterns:     []string{"identity/*"},
	})

	t.Run("namespace exclusion", func(t *testing.T) {
		excluded, reason := engine.IsExcluded(&vpb.Secret{Path: "a"}, &vpb.Namespace{Path: "admin/core"}, &vpb.Mount{Path: "secret/"})
		require.True(t, excluded)
		require.Contains(t, reason, "namespace")
	})

	t.Run("path exclusion", func(t *testing.T) {
		excluded, reason := engine.IsExcluded(&vpb.Secret{Path: "bootstrap/token"}, &vpb.Namespace{Path: "prod/app"}, &vpb.Mount{Path: "secret/"})
		require.True(t, excluded)
		require.Contains(t, reason, "path")
	})

	t.Run("mount exclusion", func(t *testing.T) {
		excluded, reason := engine.IsExcluded(&vpb.Secret{Path: "service/config"}, &vpb.Namespace{Path: "prod/app"}, &vpb.Mount{Path: "identity/"})
		require.True(t, excluded)
		require.Contains(t, reason, "mount")
	})

	t.Run("not excluded", func(t *testing.T) {
		excluded, reason := engine.IsExcluded(&vpb.Secret{Path: "service/config"}, &vpb.Namespace{Path: "prod/app"}, &vpb.Mount{Path: "secret/"})
		require.False(t, excluded)
		require.Empty(t, reason)
	})
}
