// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package audit

import (
	"strings"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type Extractor struct{}

func NewExtractor() *Extractor {
	return &Extractor{}
}

func (e *Extractor) Extract(event AuditEvent) *vpb.AccessRecord {
	if event.Type != "response" {
		return nil
	}
	if !isKVMount(event.Request.MountType) {
		return nil
	}

	secretPath, mountPath, ok := extractSecretPath(event.Request.Path)
	if !ok {
		return nil
	}

	accessType := determineAccessType(event.Request.Operation)
	if accessType == "" {
		return nil
	}

	return &vpb.AccessRecord{
		NamespacePath: event.Request.Namespace.Path,
		MountPath:     mountPath,
		MountAccessor: event.Request.MountAccessor,
		SecretPath:    secretPath,
		LastAccessed:  event.Time.UTC().Format(time.RFC3339),
		AccessType:    accessType,
		AccessCount:   1,
	}
}

func extractSecretPath(path string) (secretPath, mountPath string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 2 {
		return "", "", false
	}

	if apiIdx := kvV2APISegmentIndex(parts); apiIdx > 0 {
		if apiIdx+1 >= len(parts) {
			return "", "", false
		}
		mountPath = strings.Join(parts[:apiIdx], "/") + "/"
		secretPath = strings.Join(parts[apiIdx+1:], "/")
		if secretPath == "" {
			return "", "", false
		}
		return secretPath, mountPath, true
	}

	secretPath = strings.Join(parts[1:], "/")
	if secretPath == "" {
		return "", "", false
	}
	return secretPath, parts[0] + "/", true
}

func kvV2APISegmentIndex(parts []string) int {
	for i, part := range parts {
		switch part {
		case "data", "metadata", "subkeys":
			return i
		}
	}
	return -1
}

func determineAccessType(operation string) string {
	switch operation {
	case "read":
		return "read"
	case "create", "update":
		return "write"
	case "delete":
		return "delete"
	default:
		return ""
	}
}

func isKVMount(mountType string) bool {
	return mountType == "kv" || mountType == "generic"
}
