package discovery

import (
	"context"
	"fmt"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	api "github.com/hashicorp/vault/api"
)

type VaultClient interface {
	Address() string
	ListNamespaces(ctx context.Context) ([]string, error)
	WithNamespace(namespace string) (VaultClient, error)
	ListMounts(ctx context.Context) (map[string]*api.MountOutput, error)
	ListSecrets(ctx context.Context, path string) (*api.Secret, error)
}

type vaultClientAdapter struct {
	client *vaultpkg.Client
}

func NewVaultClientAdapter(c *vaultpkg.Client) VaultClient {
	return &vaultClientAdapter{client: c}
}

func (a *vaultClientAdapter) Address() string { return a.client.Address() }
func (a *vaultClientAdapter) ListNamespaces(ctx context.Context) ([]string, error) {
	return a.client.ListNamespaces(ctx)
}
func (a *vaultClientAdapter) WithNamespace(namespace string) (VaultClient, error) {
	c, err := a.client.WithNamespace(namespace)
	if err != nil {
		return nil, err
	}
	return &vaultClientAdapter{client: c}, nil
}
func (a *vaultClientAdapter) ListMounts(ctx context.Context) (map[string]*api.MountOutput, error) {
	return a.client.ListMounts(ctx)
}
func (a *vaultClientAdapter) ListSecrets(ctx context.Context, p string) (*api.Secret, error) {
	return a.client.ListSecrets(ctx, p)
}

type Engine struct {
	client      VaultClient
	rateLimiter *ratelimit.Limiter
	workers     int
	progress    *ProgressTracker
}

type Config struct {
	Client      VaultClient
	RateLimiter *ratelimit.Limiter
	Workers     int
}

func NewEngine(config Config) *Engine {
	if config.Workers <= 0 {
		config.Workers = 10
	}
	if config.RateLimiter == nil {
		config.RateLimiter = ratelimit.New(100)
	}

	return &Engine{
		client:      config.Client,
		rateLimiter: config.RateLimiter,
		workers:     config.Workers,
		progress:    NewProgressTracker(),
	}
}

func (e *Engine) Discover(ctx context.Context) (*vpb.Inventory, error) {
	if e.client == nil {
		return nil, fmt.Errorf("discovery client is required")
	}

	e.progress.Start()
	defer e.progress.Stop()

	inventory := &vpb.Inventory{
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		VaultAddress: e.client.Address(),
		Namespaces:   []*vpb.Namespace{},
		Stats:        &vpb.InventoryStats{},
	}

	namespaces, err := e.discoverNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover namespaces: %w", err)
	}

	inventory.Namespaces = namespaces
	e.calculateStats(inventory)
	return inventory, nil
}

func (e *Engine) calculateStats(inventory *vpb.Inventory) {
	stats := &vpb.InventoryStats{}
	for _, ns := range inventory.Namespaces {
		stats.NamespaceCount++
		for _, mount := range ns.Mounts {
			stats.MountCount++
			secretCount := int32(len(mount.Secrets))
			stats.SecretCount += secretCount
			if mount.Version == 1 {
				stats.KvV1Count += secretCount
			} else {
				stats.KvV2Count += secretCount
			}
		}
	}
	inventory.Stats = stats
}
