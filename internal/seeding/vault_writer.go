// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package seeding

import (
	"context"
	"fmt"
	"path"
	"strings"

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
	nsClient, err := w.client.WithNamespace(composeNamespace(w.client.Namespace(), namespace))
	if err != nil {
		return err
	}
	return nsClient.EnableKVMount(ctx, mountPath, kvVersion)
}

func (w *VaultClientWriter) WriteKVSecret(ctx context.Context, namespace, mountPath, secretPath string, data map[string]any, kvVersion int) error {
	nsClient, err := w.client.WithNamespace(composeNamespace(w.client.Namespace(), namespace))
	if err != nil {
		return err
	}
	return nsClient.WriteKVSecret(ctx, mountPath, secretPath, data, kvVersion)
}

// GetMountKVVersions returns the KV version (1 or 2) for each path in
// mountPaths within the given namespace. It returns an error if any mount is
// absent from Vault or is not a KV mount.
func (w *VaultClientWriter) GetMountKVVersions(ctx context.Context, namespace string, mountPaths []string) (map[string]int, error) {
	nsClient, err := w.client.WithNamespace(composeNamespace(w.client.Namespace(), namespace))
	if err != nil {
		return nil, err
	}
	versions := make(map[string]int, len(mountPaths))
	for _, mp := range mountPaths {
		v, err := nsClient.KVVersion(ctx, mp)
		if err != nil {
			return nil, fmt.Errorf("discover kv version for mount %q in namespace %q: %w", mp, namespace, err)
		}
		versions[mp] = v
	}
	return versions, nil
}

func composeNamespace(base, child string) string {
	trim := func(s string) string {
		return strings.Trim(strings.TrimSpace(s), "/")
	}

	b := trim(base)
	c := trim(child)

	if b == "" {
		return c
	}
	if c == "" {
		return b
	}
	if c == b || strings.HasPrefix(c, b+"/") {
		return c
	}

	return path.Join(b, c)
}
