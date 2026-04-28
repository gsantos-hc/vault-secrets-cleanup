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
	if err := e.listSecretsRecursive(ctx, client, basePath, "", &secrets); err != nil {
		return nil, err
	}
	e.progress.AddSecrets(len(secrets))
	e.emitProgress()
	return secrets, nil
}

func (e *Engine) listSecretsRecursive(ctx context.Context, client VaultClient, basePath, subPath string, secrets *[]*vpb.Secret) error {
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return err
	}

	fullPath := strings.Trim(path.Join(basePath, subPath), "/")
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
		relPath := path.Join(subPath, keyStr)
		if strings.HasSuffix(keyStr, "/") {
			err := e.listSecretsRecursive(ctx, client, basePath, relPath, secrets)
			if err != nil {
				e.progress.LogError(fmt.Sprintf("path %s: %v", relPath, err))
				e.emitProgress()
				continue
			}
			continue
		}

		*secrets = append(*secrets, &vpb.Secret{
			Path:         relPath,
			DiscoveredAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	return nil
}
