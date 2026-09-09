// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package seeding

import (
	"context"
	"fmt"
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
	composed, err := composeNamespace(w.client.Namespace(), namespace)
	if err != nil {
		return err
	}
	nsClient, err := w.client.WithNamespace(composed)
	if err != nil {
		return err
	}
	return nsClient.EnableKVMount(ctx, mountPath, kvVersion)
}

func (w *VaultClientWriter) WriteKVSecret(ctx context.Context, namespace, mountPath, secretPath string, data map[string]any, kvVersion int) error {
	composed, err := composeNamespace(w.client.Namespace(), namespace)
	if err != nil {
		return err
	}
	nsClient, err := w.client.WithNamespace(composed)
	if err != nil {
		return err
	}
	return nsClient.WriteKVSecret(ctx, mountPath, secretPath, data, kvVersion)
}

// GetMountKVVersions returns the KV version (1 or 2) for each path in
// mountPaths within the given namespace. It performs a single sys/mounts
// listing and derives all requested versions from that one response, rather
// than issuing one listing per mount.
func (w *VaultClientWriter) GetMountKVVersions(ctx context.Context, namespace string, mountPaths []string) (map[string]int, error) {
	composed, err := composeNamespace(w.client.Namespace(), namespace)
	if err != nil {
		return nil, err
	}
	nsClient, err := w.client.WithNamespace(composed)
	if err != nil {
		return nil, err
	}
	versions, err := nsClient.KVVersions(ctx, mountPaths)
	if err != nil {
		return nil, fmt.Errorf("discover kv versions in namespace %q: %w", namespace, err)
	}
	return versions, nil
}

// composeNamespace joins base and child into a single Vault namespace path.
// It returns an error if child contains any ".." component, which would allow
// a caller-supplied value to escape the configured base namespace.
func composeNamespace(base, child string) (string, error) {
	trim := func(s string) string {
		return strings.Trim(strings.TrimSpace(s), "/")
	}

	b := trim(base)
	c := trim(child)

	// Reject any segment that is ".." to prevent traversal out of the
	// configured namespace prefix regardless of how path.Join would resolve it.
	for _, seg := range strings.Split(c, "/") {
		if seg == ".." {
			return "", fmt.Errorf("invalid namespace %q: path traversal not allowed", child)
		}
	}

	if b == "" {
		return c, nil
	}
	if c == "" {
		return b, nil
	}
	if c == b || strings.HasPrefix(c, b+"/") {
		return c, nil
	}

	return b + "/" + c, nil
}
