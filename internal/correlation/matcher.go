package correlation

import vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"

type MatchingStrategy string

const (
	MatchingStrategyAccessor  MatchingStrategy = "accessor"
	MatchingStrategyPathBased MatchingStrategy = "path-based"
)

type Matcher struct {
	strategy MatchingStrategy
}

func NewMatcher(strategy MatchingStrategy) *Matcher {
	if strategy == "" {
		strategy = MatchingStrategyAccessor
	}

	return &Matcher{strategy: strategy}
}

func (m *Matcher) Match(secret *vpb.Secret, namespace *vpb.Namespace, mount *vpb.Mount, access *vpb.AccessRecord) bool {
	if secret == nil || namespace == nil || mount == nil || access == nil {
		return false
	}

	switch m.strategy {
	case MatchingStrategyPathBased:
		return namespace.Path == access.NamespacePath &&
			mount.Path == access.MountPath &&
			secret.Path == access.SecretPath
	case MatchingStrategyAccessor:
		fallthrough
	default:
		return mount.Accessor == access.MountAccessor &&
			secret.Path == access.SecretPath
	}
}
