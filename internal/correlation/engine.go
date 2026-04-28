package correlation

import (
	"fmt"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type Config struct {
	MatchingStrategy MatchingStrategy
	OnProgress       func(ProgressSnapshot)
}

type ProgressSnapshot struct {
	Completed int
	Total     int
}

type Engine struct {
	matcher    *Matcher
	onProgress func(ProgressSnapshot)
}

func NewEngine(config Config) *Engine {
	return &Engine{matcher: NewMatcher(config.MatchingStrategy), onProgress: config.OnProgress}
}

func (e *Engine) Correlate(inventory *vpb.Inventory, accessData *vpb.AccessData) (map[string]*vpb.AccessRecord, error) {
	result := make(map[string]*vpb.AccessRecord)
	if inventory == nil || accessData == nil {
		return result, nil
	}

	total := 0
	for _, namespace := range inventory.Namespaces {
		for _, mount := range namespace.Mounts {
			total += len(mount.Secrets)
		}
	}
	completed := 0

	for _, namespace := range inventory.Namespaces {
		for _, mount := range namespace.Mounts {
			for _, secret := range mount.Secrets {
				for _, record := range accessData.Records {
					if e.matcher.Match(secret, namespace, mount, record) {
						result[BuildKey(namespace, mount, secret)] = record
						break
					}
				}
				completed++
				e.emitProgress(completed, total)
			}
		}
	}

	return result, nil
}

func (e *Engine) emitProgress(completed, total int) {
	if e.onProgress == nil {
		return
	}
	e.onProgress(ProgressSnapshot{Completed: completed, Total: total})
}

func BuildKey(namespace *vpb.Namespace, mount *vpb.Mount, secret *vpb.Secret) string {
	if namespace == nil || mount == nil || secret == nil {
		return ""
	}

	return fmt.Sprintf("%s|%s|%s", namespace.Path, mount.Accessor, secret.Path)
}
