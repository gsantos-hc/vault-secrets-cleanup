package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/seeding"
	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
)

func main() {
	var (
		configPath      string
		vaultAddr       string
		vaultToken      string
		namespaceCount  int
		totalSecrets    int
		kv2Probability  float64
		randomSeed      int64
		namespacePrefix string
		mountPrefix     string
		dryRun          bool
		maxAttempts     int
	)

	flag.StringVar(&configPath, "config", "", "Path to YAML configuration file")
	flag.StringVar(&vaultAddr, "vault-addr", "", "Vault address (overrides config/env)")
	flag.StringVar(&vaultToken, "vault-token", "", "Vault token (overrides config/env)")
	flag.IntVar(&namespaceCount, "namespaces", 10, "Number of namespaces to create")
	flag.IntVar(&totalSecrets, "total-secrets", 1000, "Total number of secrets to create across the cluster")
	flag.Float64Var(&kv2Probability, "kv2-probability", 0.9, "Probability that a generated mount uses KV v2")
	flag.Int64Var(&randomSeed, "seed", 0, "Random seed (0 uses current time)")
	flag.StringVar(&namespacePrefix, "namespace-prefix", "seed", "Prefix for generated namespace names")
	flag.StringVar(&mountPrefix, "mount-prefix", "seed", "Prefix for generated mount names")
	flag.BoolVar(&dryRun, "dry-run", false, "Plan operations without writing to Vault")
	flag.IntVar(&maxAttempts, "max-attempts", 3, "Maximum write retry attempts per operation")
	flag.Parse()

	cfg, err := config.Load(configPath, map[string]any{
		"vault.address": vaultAddr,
		"vault.token":   vaultToken,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	client, err := vaultpkg.NewClient(vaultpkg.Config{
		Address: cfg.Vault.Address,
		Token:   cfg.Vault.Token,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create vault client: %v\n", err)
		os.Exit(1)
	}

	if randomSeed == 0 {
		randomSeed = time.Now().UnixNano()
	}

	engine := seeding.NewEngine(seeding.Config{
		Writer:          seeding.NewVaultClientWriter(client),
		RateLimiter:     ratelimit.NewWithBurst(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst),
		Retry:           retry.Config{MaxAttempts: maxAttempts},
		NamespaceCount:  namespaceCount,
		TotalSecrets:    totalSecrets,
		KV2Probability:  kv2Probability,
		RandomSeed:      randomSeed,
		NamespacePrefix: namespacePrefix,
		MountPrefix:     mountPrefix,
		DryRun:          dryRun,
	})

	result, err := engine.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed run failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("planned namespaces=%d mounts=%d secrets=%d\n", result.PlannedNamespaces, result.PlannedMounts, result.PlannedSecrets)
	fmt.Printf("created namespaces=%d mounts=%d secrets=%d\n", result.NamespacesCreated, result.MountsCreated, result.SecretsWritten)
	fmt.Printf("seed=%d dry_run=%t\n", randomSeed, dryRun)
}
