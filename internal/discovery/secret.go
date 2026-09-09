// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package discovery

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func (e *Engine) discoverSecrets(ctx context.Context, client VaultClient, ns *vpb.Namespace, mount *vpb.Mount) ([]*vpb.Secret, error) {
	_ = ns
	basePath := strings.TrimSuffix(mount.Path, "/")
	if mount.Version == 2 {
		basePath = path.Join(basePath, "metadata")
	}

	secrets := make([]*vpb.Secret, 0)
	seenSecrets := make(map[string]struct{})
	visitedSubpaths := make(map[string]struct{})
	if err := e.listSecretsRecursive(ctx, client, basePath, "", &secrets, seenSecrets, visitedSubpaths); err != nil {
		return nil, err
	}
	e.progress.AddSecrets(len(secrets))
	e.emitProgress()
	return secrets, nil
}

func (e *Engine) listSecretsRecursive(ctx context.Context, client VaultClient, basePath, subPath string, secrets *[]*vpb.Secret, seenSecrets, visitedSubpaths map[string]struct{}) error {
	normalizedSubPath := normalizeSecretPath(subPath)
	if _, seen := visitedSubpaths[normalizedSubPath]; seen {
		return nil
	}
	visitedSubpaths[normalizedSubPath] = struct{}{}

	if err := e.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	fullPath := strings.Trim(path.Join(basePath, normalizedSubPath), "/")
	secret, err := client.ListSecrets(ctx, fullPath)
	if err != nil {
		return fmt.Errorf("failed to list %s: %w", fullPath, err)
	}
	if secret == nil || secret.Data == nil {
		return nil
	}

	rawKeys, ok := secret.Data["keys"]
	if !ok {
		return nil
	}

	keys, ok := rawKeys.([]any)
	if !ok {
		return nil
	}

	for _, key := range keys {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}

		trimmedKey := strings.TrimSpace(keyStr)
		if trimmedKey == "" {
			continue
		}

		isFolder := strings.HasSuffix(trimmedKey, "/")
		normalizedKey := strings.Trim(trimmedKey, "/")
		if normalizedKey == "" {
			continue
		}

		relPath := normalizeSecretPath(path.Join(normalizedSubPath, normalizedKey))
		if isFolder {
			err := e.listSecretsRecursive(ctx, client, basePath, relPath, secrets, seenSecrets, visitedSubpaths)
			if err != nil {
				e.progress.LogError(fmt.Sprintf("path %s: %v", relPath, err))
				e.emitProgress()
			}
			continue
		}

		if _, seen := seenSecrets[relPath]; seen {
			continue
		}
		seenSecrets[relPath] = struct{}{}

		*secrets = append(*secrets, &vpb.Secret{
			Path:         relPath,
			DiscoveredAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	return nil
}

func normalizeSecretPath(p string) string {
	trimmed := strings.TrimSpace(p)
	if trimmed == "" {
		return ""
	}
	return strings.Trim(path.Clean(trimmed), "/")
}
