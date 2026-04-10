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
		NamespaceId:   event.Request.Namespace.ID,
		MountPath:     mountPath,
		MountAccessor: event.Request.MountAccessor,
		SecretPath:    secretPath,
		LastAccessed:  event.Time.UTC().Format(time.RFC3339),
		AccessType:    accessType,
		AccessCount:   1,
	}
}

func extractSecretPath(path string) (secretPath, mountPath string, ok bool) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return "", "", false
	}
	if len(parts) >= 3 && (parts[1] == "data" || parts[1] == "metadata") {
		return strings.Join(parts[2:], "/"), parts[0] + "/", true
	}
	return strings.Join(parts[1:], "/"), parts[0] + "/", true
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
