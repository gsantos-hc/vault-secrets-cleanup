// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package deletion

import (
	"context"
	"fmt"
	"path"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type SecretDeleter interface {
	KVVersion(ctx context.Context, namespacePath, mountPath string) (int, error)
	DeleteKVv1(ctx context.Context, namespacePath, mountPath, secretPath string) error
	DeleteKVv2(ctx context.Context, namespacePath, mountPath, secretPath string) error
}

type Executor struct {
	deleter     SecretDeleter
	retryConfig retry.Config
	dryRun      bool
}

type ExecutorConfig struct {
	Deleter     SecretDeleter
	RetryConfig retry.Config
	DryRun      bool
}

func NewExecutor(config ExecutorConfig) *Executor {
	return &Executor{
		deleter:     config.Deleter,
		retryConfig: config.RetryConfig,
		dryRun:      config.DryRun,
	}
}

func (e *Executor) Delete(ctx context.Context, action *vpb.SecretAction) error {
	if action == nil {
		return fmt.Errorf("action is required")
	}
	if e.deleter == nil {
		return fmt.Errorf("deleter is required")
	}
	if e.dryRun {
		return nil
	}

	kvVersion, err := e.deleter.KVVersion(ctx, action.NamespacePath, action.MountPath)
	if err != nil {
		return fmt.Errorf("determine kv version: %w", err)
	}

	return retry.DoWithRetryable(ctx, e.retryConfig, func() error {
		switch kvVersion {
		case 1:
			return e.deleter.DeleteKVv1(ctx, action.NamespacePath, action.MountPath, action.SecretPath)
		case 2:
			return e.deleter.DeleteKVv2(ctx, action.NamespacePath, action.MountPath, action.SecretPath)
		default:
			return fmt.Errorf("unsupported kv version %d for mount %q", kvVersion, action.MountPath)
		}
	}, retry.IsRetryable)
}

type VaultSecretDeleter struct {
	client *vaultpkg.Client
}

func NewVaultSecretDeleter(client *vaultpkg.Client) *VaultSecretDeleter {
	return &VaultSecretDeleter{client: client}
}

func (d *VaultSecretDeleter) KVVersion(ctx context.Context, namespacePath, mountPath string) (int, error) {
	client, err := d.scopedClient(namespacePath)
	if err != nil {
		return 0, err
	}
	return client.KVVersion(ctx, mountPath)
}

func (d *VaultSecretDeleter) DeleteKVv1(ctx context.Context, namespacePath, mountPath, secretPath string) error {
	client, err := d.scopedClient(namespacePath)
	if err != nil {
		return err
	}

	fullPath := path.Join(mountPath, secretPath)
	if err := client.Delete(ctx, fullPath); err != nil {
		return fmt.Errorf("delete kv v1 secret %q: %w", fullPath, err)
	}
	return nil
}

func (d *VaultSecretDeleter) DeleteKVv2(ctx context.Context, namespacePath, mountPath, secretPath string) error {
	client, err := d.scopedClient(namespacePath)
	if err != nil {
		return err
	}

	metadataPath := path.Join(mountPath, "metadata", secretPath)
	if err := client.Delete(ctx, metadataPath); err != nil {
		return fmt.Errorf("delete kv v2 secret metadata %q: %w", metadataPath, err)
	}
	return nil
}

func (d *VaultSecretDeleter) scopedClient(namespacePath string) (*vaultpkg.Client, error) {
	if d.client == nil {
		return nil, fmt.Errorf("vault client is required")
	}
	if namespacePath == "" {
		return d.client, nil
	}

	nsClient, err := d.client.WithNamespace(namespacePath)
	if err != nil {
		return nil, fmt.Errorf("create namespace client for %q: %w", namespacePath, err)
	}
	return nsClient, nil
}
