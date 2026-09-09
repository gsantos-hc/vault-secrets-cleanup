// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package discovery

import (
	"context"
	"fmt"
	"sync"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func (e *Engine) discoverNamespaces(ctx context.Context) ([]*vpb.Namespace, error) {
	namespacePaths := []string{""}
	if err := e.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	childPaths, err := e.client.ListNamespaces(ctx)
	if err != nil {
		childPaths = nil
	}
	namespacePaths = append(namespacePaths, childPaths...)

	type namespaceJob struct {
		index int
		path  string
	}
	type namespaceResult struct {
		index  int
		ns     *vpb.Namespace
		client VaultClient
		mounts []*vpb.Mount
		err    error
	}

	workerCount := e.workers
	if workerCount <= 0 {
		workerCount = 1
	}
	if workerCount > len(namespacePaths) {
		workerCount = len(namespacePaths)
	}

	jobs := make(chan namespaceJob)
	results := make(chan namespaceResult, len(namespacePaths))

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				ns, client, mounts, err := e.discoverNamespaceMounts(ctx, job.path)
				results <- namespaceResult{index: job.index, ns: ns, client: client, mounts: mounts, err: err}
			}
		}()
	}

	for i, nsPath := range namespacePaths {
		jobs <- namespaceJob{index: i, path: nsPath}
	}
	close(jobs)

	go func() {
		wg.Wait()
		close(results)
	}()

	orderedResults := make([]*namespaceResult, len(namespacePaths))
	for result := range results {
		res := result
		if result.err != nil {
			e.progress.LogError(fmt.Sprintf("namespace %s: %v", namespacePaths[result.index], result.err))
			e.emitProgress()
			continue
		}
		e.progress.AddNamespace()
		e.emitProgress()
		orderedResults[result.index] = &res
	}

	namespaces := make([]*vpb.Namespace, 0, len(namespacePaths))
	mountJobs := make([]mountDiscoveryJob, 0)
	for _, result := range orderedResults {
		if result == nil {
			continue
		}
		result.ns.Mounts = []*vpb.Mount{}
		namespaces = append(namespaces, result.ns)
		for mountIndex, mount := range result.mounts {
			mountJobs = append(mountJobs, mountDiscoveryJob{
				namespace:  result.ns,
				client:     result.client,
				mount:      mount,
				mountIndex: mountIndex,
			})
		}
	}

	if err := e.discoverMountSecrets(ctx, mountJobs); err != nil {
		return nil, err
	}

	return namespaces, nil
}

func (e *Engine) discoverNamespaceMounts(ctx context.Context, path string) (*vpb.Namespace, VaultClient, []*vpb.Mount, error) {
	nsClient := e.client
	if path != "" {
		var err error
		nsClient, err = e.client.WithNamespace(path)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("create namespace client: %w", err)
		}
	}

	ns := &vpb.Namespace{Path: path, Mounts: []*vpb.Mount{}}
	mounts, err := e.discoverMountMetadataWithClient(ctx, nsClient, ns)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to discover mounts: %w", err)
	}
	return ns, nsClient, mounts, nil
}
