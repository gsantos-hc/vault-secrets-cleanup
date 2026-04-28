package discovery

import (
	"context"
	"fmt"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func (e *Engine) discoverNamespaces(ctx context.Context) ([]*vpb.Namespace, error) {
	root := &vpb.Namespace{Path: "", Mounts: []*vpb.Mount{}}

	mounts, err := e.discoverMounts(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("failed to discover root mounts: %w", err)
	}
	root.Mounts = mounts
	e.progress.AddNamespace()
	e.emitProgress()

	namespaces := []*vpb.Namespace{root}

	if err := e.rateLimiter.Wait(ctx); err != nil {
		return nil, err
	}
	childPaths, err := e.client.ListNamespaces(ctx)
	if err != nil {
		return namespaces, nil
	}

	for _, path := range childPaths {
		ns, err := e.discoverNamespace(ctx, path)
		if err != nil {
			e.progress.LogError(fmt.Sprintf("namespace %s: %v", path, err))
			e.emitProgress()
			continue
		}
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

func (e *Engine) discoverNamespace(ctx context.Context, path string) (*vpb.Namespace, error) {
	nsClient, err := e.client.WithNamespace(path)
	if err != nil {
		return nil, fmt.Errorf("create namespace client: %w", err)
	}

	ns := &vpb.Namespace{Path: path, Mounts: []*vpb.Mount{}}
	mounts, err := e.discoverMountsWithClient(ctx, nsClient, ns)
	if err != nil {
		return nil, fmt.Errorf("failed to discover mounts: %w", err)
	}
	ns.Mounts = mounts
	e.progress.AddNamespace()
	e.emitProgress()
	return ns, nil
}
