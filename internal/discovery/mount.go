package discovery

import (
	"context"
	"fmt"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	api "github.com/hashicorp/vault/api"
)

func (e *Engine) discoverMounts(ctx context.Context, ns *vpb.Namespace) ([]*vpb.Mount, error) {
	client := e.client
	if ns.Path != "" {
		nsClient, err := e.client.WithNamespace(ns.Path)
		if err != nil {
			return nil, err
		}
		client = nsClient
	}
	return e.discoverMountsWithClient(ctx, client, ns)
}

func (e *Engine) discoverMountsWithClient(ctx context.Context, client VaultClient, ns *vpb.Namespace) ([]*vpb.Mount, error) {
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	mounts, err := client.ListMounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list mounts: %w", err)
	}

	kvMounts := make([]*vpb.Mount, 0)
	for path, mount := range mounts {
		if !isKVMount(mount.Type) {
			continue
		}

		kvMount := &vpb.Mount{
			Path:     path,
			Accessor: mount.Accessor,
			Type:     mount.Type,
			Version:  getKVVersion(mount),
			Secrets:  []*vpb.Secret{},
		}

		secrets, err := e.discoverSecrets(ctx, client, ns, kvMount)
		if err != nil {
			e.progress.LogError(fmt.Sprintf("mount %s/%s: %v", ns.Path, path, err))
			e.emitProgress()
			continue
		}
		kvMount.Secrets = secrets
		kvMounts = append(kvMounts, kvMount)
		e.progress.AddMount()
		e.emitProgress()
	}

	return kvMounts, nil
}

func isKVMount(mountType string) bool {
	return mountType == "kv" || mountType == "generic"
}

func getKVVersion(mount *api.MountOutput) int32 {
	if mount.Options != nil {
		if version, ok := mount.Options["version"]; ok && version == "2" {
			return 2
		}
	}
	return 1
}
