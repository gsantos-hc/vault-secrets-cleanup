package correlation

import (
	"fmt"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type Config struct {
	MatchingStrategy MatchingStrategy
}

type Engine struct {
	matcher *Matcher
}

func NewEngine(config Config) *Engine {
	return &Engine{matcher: NewMatcher(config.MatchingStrategy)}
}

func (e *Engine) Correlate(inventory *vpb.Inventory, accessData *vpb.AccessData) (map[string]*vpb.AccessRecord, error) {
	result := make(map[string]*vpb.AccessRecord)
	if inventory == nil || accessData == nil {
		return result, nil
	}

	for _, namespace := range inventory.Namespaces {
		for _, mount := range namespace.Mounts {
			for _, secret := range mount.Secrets {
				for _, record := range accessData.Records {
					if e.matcher.Match(secret, namespace, mount, record) {
						result[BuildKey(namespace, mount, secret)] = record
						break
					}
				}
			}
		}
	}

	return result, nil
}

func BuildKey(namespace *vpb.Namespace, mount *vpb.Mount, secret *vpb.Secret) string {
	if namespace == nil || mount == nil || secret == nil {
		return ""
	}

	return fmt.Sprintf("%s|%s|%s", namespace.Path, mount.Accessor, secret.Path)
}
