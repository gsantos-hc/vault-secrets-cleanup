package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/discovery"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/spf13/cobra"
	gproto "google.golang.org/protobuf/proto"
)

type discoverOptions struct {
	output  string
	workers int
}

type discoverEngine interface {
	Discover(ctx context.Context) (*vpb.Inventory, error)
}

var createDiscoverEngine = func(cfg *config.Config, workers int) (discoverEngine, error) {
	client, err := vaultpkg.NewClient(vaultpkg.Config{
		Address: cfg.Vault.Address,
		Token:   cfg.Vault.Token,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create vault client: %w", err)
	}

	eng := discovery.NewEngine(discovery.Config{
		Client:      discovery.NewVaultClientAdapter(client),
		RateLimiter: ratelimit.New(cfg.RateLimit.RequestsPerSecond),
		Workers:     workers,
	})
	return wrappedDiscoverEngine{engine: eng}, nil
}

type wrappedDiscoverEngine struct {
	engine *discovery.Engine
}

func (w wrappedDiscoverEngine) Discover(ctx context.Context) (*vpb.Inventory, error) {
	return w.engine.Discover(ctx)
}

func newDiscoverCmd() *cobra.Command {
	opts := discoverOptions{}
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover all secrets across Vault namespaces",
		Long:  "Enumerate namespaces, KV mounts, and secrets; export inventory as Protocol Buffers.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiscover(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVarP(&opts.output, "output", "o", "inventory.pb", "Output file for inventory protobuf")
	cmd.Flags().IntVar(&opts.workers, "workers", 10, "Number of concurrent workers")
	return cmd
}

func runDiscover(ctx context.Context, opts discoverOptions) error {
	cfg := GetConfig()
	if cfg == nil {
		return fmt.Errorf("configuration not loaded")
	}

	engine, err := createDiscoverEngine(cfg, opts.workers)
	if err != nil {
		return err
	}

	inv, err := engine.Discover(ctx)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	data, err := gproto.Marshal(inv)
	if err != nil {
		return fmt.Errorf("failed to marshal inventory: %w", err)
	}

	if err := os.WriteFile(opts.output, data, 0o644); err != nil {
		return fmt.Errorf("failed to write inventory: %w", err)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("discovery complete", "output", opts.output)
	}

	return nil
}
