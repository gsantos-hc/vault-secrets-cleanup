package seeding

import (
	"context"

	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
)

type VaultClientWriter struct {
	client *vaultpkg.Client
}

func NewVaultClientWriter(client *vaultpkg.Client) *VaultClientWriter {
	return &VaultClientWriter{client: client}
}

func (w *VaultClientWriter) CreateNamespace(ctx context.Context, namespace string) error {
	return w.client.CreateNamespace(ctx, namespace)
}

func (w *VaultClientWriter) EnableKVMount(ctx context.Context, namespace, mountPath string, kvVersion int) error {
	nsClient, err := w.client.WithNamespace(namespace)
	if err != nil {
		return err
	}
	return nsClient.EnableKVMount(ctx, mountPath, kvVersion)
}

func (w *VaultClientWriter) WriteKVSecret(ctx context.Context, namespace, mountPath, secretPath string, data map[string]any, kvVersion int) error {
	nsClient, err := w.client.WithNamespace(namespace)
	if err != nil {
		return err
	}
	return nsClient.WriteKVSecret(ctx, mountPath, secretPath, data, kvVersion)
}
