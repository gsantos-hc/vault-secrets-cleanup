// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package planning

import (
	"fmt"
	"path/filepath"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type ExclusionConfig struct {
	NamespacePatterns []string
	PathPatterns      []string
	MountPatterns     []string
}

type ExclusionEngine struct {
	namespacePatterns []string
	pathPatterns      []string
	mountPatterns     []string
}

func NewExclusionEngine(config ExclusionConfig) *ExclusionEngine {
	return &ExclusionEngine{
		namespacePatterns: config.NamespacePatterns,
		pathPatterns:      config.PathPatterns,
		mountPatterns:     config.MountPatterns,
	}
}

func (e *ExclusionEngine) IsExcluded(secret *vpb.Secret, namespace *vpb.Namespace, mount *vpb.Mount) (bool, string) {
	for _, pattern := range e.namespacePatterns {
		if matches(pattern, namespace.GetPath()) {
			return true, fmt.Sprintf("namespace matches exclusion pattern: %s", pattern)
		}
	}

	fullPath := mount.GetPath() + secret.GetPath()
	for _, pattern := range e.pathPatterns {
		if matches(pattern, fullPath) {
			return true, fmt.Sprintf("path matches exclusion pattern: %s", pattern)
		}
	}

	for _, pattern := range e.mountPatterns {
		if matches(pattern, mount.GetPath()) {
			return true, fmt.Sprintf("mount matches exclusion pattern: %s", pattern)
		}
	}

	return false, ""
}

func matches(pattern string, value string) bool {
	matched, err := filepath.Match(pattern, value)
	if err != nil {
		return false
	}
	return matched
}
