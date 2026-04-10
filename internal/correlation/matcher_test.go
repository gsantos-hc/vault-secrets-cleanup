package correlation

import (
	"testing"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestMatcherMatch(t *testing.T) {
	secret := &vpb.Secret{Path: "app/config"}
	namespace := &vpb.Namespace{Path: "prod/app"}
	mount := &vpb.Mount{Path: "secret/", Accessor: "kv_1"}
	access := &vpb.AccessRecord{
		NamespacePath: "prod/app",
		MountPath:     "secret/",
		MountAccessor: "kv_1",
		SecretPath:    "app/config",
	}

	t.Run("strict match succeeds", func(t *testing.T) {
		matcher := NewMatcher(MatchingStrategyStrict)
		require.True(t, matcher.Match(secret, namespace, mount, access))
	})

	t.Run("strict match fails with different accessor", func(t *testing.T) {
		matcher := NewMatcher(MatchingStrategyStrict)
		modified := &vpb.AccessRecord{
			NamespacePath: access.NamespacePath,
			MountPath:     access.MountPath,
			MountAccessor: "kv_2",
			SecretPath:    access.SecretPath,
		}
		require.False(t, matcher.Match(secret, namespace, mount, modified))
	})

	t.Run("strict match fails with different namespace path", func(t *testing.T) {
		matcher := NewMatcher(MatchingStrategyStrict)
		modified := &vpb.AccessRecord{
			NamespacePath: "prod/other",
			MountPath:     access.MountPath,
			MountAccessor: access.MountAccessor,
			SecretPath:    access.SecretPath,
		}
		require.False(t, matcher.Match(secret, namespace, mount, modified))
	})

	t.Run("path based match ignores id and accessor", func(t *testing.T) {
		matcher := NewMatcher(MatchingStrategyPathBased)
		modified := &vpb.AccessRecord{
			NamespacePath: access.NamespacePath,
			MountPath:     access.MountPath,
			MountAccessor: "",
			SecretPath:    access.SecretPath,
		}
		require.True(t, matcher.Match(secret, namespace, mount, modified))
	})
}
