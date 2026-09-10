// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package discovery

import (
	"context"
	"fmt"
	"sort"
	"sync"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	api "github.com/hashicorp/vault/api"
)

type mountDiscoveryJob struct {
	namespace  *vpb.Namespace
	client     VaultClient
	mount      *vpb.Mount
	mountIndex int
}

func (e *Engine) discoverMountMetadataWithClient(ctx context.Context, client VaultClient, ns *vpb.Namespace) ([]*vpb.Mount, error) {
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
		kvMounts = append(kvMounts, kvMount)
	}

	return kvMounts, nil
}

func (e *Engine) discoverMountSecrets(ctx context.Context, jobs []mountDiscoveryJob) error {
	if len(jobs) == 0 {
		return nil
	}

	workerCount := e.workers
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > len(jobs) {
		workerCount = len(jobs)
	}

	type mountSecretsResult struct {
		job     mountDiscoveryJob
		secrets []*vpb.Secret
		err     error
	}

	jobCh := make(chan mountDiscoveryJob)
	resultCh := make(chan mountSecretsResult, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				secrets, err := e.discoverSecrets(ctx, job.client, job.namespace, job.mount)
				resultCh <- mountSecretsResult{job: job, secrets: secrets, err: err}
			}
		}()
	}

	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	type indexedMount struct {
		index int
		mount *vpb.Mount
	}

	collected := make(map[*vpb.Namespace][]indexedMount)
	for result := range resultCh {
		if result.err != nil {
			e.progress.LogError(fmt.Sprintf("mount %s/%s: %v", result.job.namespace.Path, result.job.mount.Path, result.err))
			e.emitProgress()
			continue
		}

		result.job.mount.Secrets = result.secrets
		collected[result.job.namespace] = append(collected[result.job.namespace], indexedMount{index: result.job.mountIndex, mount: result.job.mount})
		e.progress.AddMount()
		e.emitProgress()
	}

	for ns, mounts := range collected {
		sort.Slice(mounts, func(i, j int) bool {
			return mounts[i].index < mounts[j].index
		})

		ns.Mounts = make([]*vpb.Mount, 0, len(mounts))
		for _, mount := range mounts {
			ns.Mounts = append(ns.Mounts, mount.mount)
		}
	}

	return nil
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
