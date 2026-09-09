// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
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
		workers         int
		namespacePrefix string
		mountPrefix     string
		skipNamespaces  bool
		skipMounts      bool
		maxFailures     int
		dryRun          bool
		maxAttempts     int
		progressMode    string
	)

	flag.StringVar(&configPath, "config", "", "Path to YAML configuration file")
	flag.StringVar(&vaultAddr, "vault-addr", "", "Vault address (overrides config/env)")
	flag.StringVar(&vaultToken, "vault-token", "", "Vault token (overrides config/env)")
	flag.IntVar(&namespaceCount, "namespaces", 10, "Number of namespaces to create")
	flag.IntVar(&totalSecrets, "total-secrets", 1000, "Total number of secrets to create across the cluster")
	flag.Float64Var(&kv2Probability, "kv2-probability", 0.9, "Probability that a generated mount uses KV v2")
	flag.Int64Var(&randomSeed, "seed", 0, "Random seed (0 uses current time)")
	flag.IntVar(&workers, "workers", 4, "Number of parallel workers to use within each seeding stage")
	flag.StringVar(&namespacePrefix, "namespace-prefix", "seed", "Prefix for generated namespace names")
	flag.StringVar(&mountPrefix, "mount-prefix", "seed", "Prefix for generated mount names")
	flag.BoolVar(&skipNamespaces, "skip-namespaces", false, "Skip creating namespaces (assume they already exist)")
	flag.BoolVar(&skipMounts, "skip-mounts", false, "Skip enabling mounts (assume they already exist)")
	flag.IntVar(&maxFailures, "max-failures", 0, "Maximum individual operation failures before stopping; 0 = stop on first, -1 = never stop")
	flag.BoolVar(&dryRun, "dry-run", false, "Plan operations without writing to Vault")
	flag.IntVar(&maxAttempts, "max-attempts", 3, "Maximum write retry attempts per operation")
	flag.StringVar(&progressMode, "progress", "auto", "Progress output mode (auto|tty|log|off)")
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

	reporter, err := newSeedProgressReporter(os.Stdout, progressMode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid progress mode: %v\n", err)
		os.Exit(1)
	}
	defer reporter.Close()

	engine := seeding.NewEngine(seeding.Config{
		Writer:          seeding.NewVaultClientWriter(client),
		RateLimiter:     ratelimit.NewWithBurst(cfg.RateLimit.RequestsPerSecond, cfg.RateLimit.Burst),
		Retry:           retry.Config{MaxAttempts: maxAttempts},
		NamespaceCount:  namespaceCount,
		TotalSecrets:    totalSecrets,
		KV2Probability:  kv2Probability,
		RandomSeed:      randomSeed,
		Workers:         workers,
		NamespacePrefix: namespacePrefix,
		MountPrefix:     mountPrefix,
		SkipNamespaces:  skipNamespaces,
		SkipMounts:      skipMounts,
		MaxFailures:     maxFailures,
		DryRun:          dryRun,
		OnProgress: func(snapshot seeding.ProgressSnapshot) {
			reporter.Emit(snapshot)
		},
	})

	result, err := engine.Run(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "seed run failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("planned namespaces=%d mounts=%d secrets=%d\n", result.PlannedNamespaces, result.PlannedMounts, result.PlannedSecrets)
	fmt.Printf("created namespaces=%d mounts=%d secrets=%d failures=%d\n", result.NamespacesCreated, result.MountsCreated, result.SecretsWritten, result.Failures)
	fmt.Printf("seed=%d dry_run=%t\n", randomSeed, dryRun)
}

type seedProgressMode string

const (
	seedProgressAuto seedProgressMode = "auto"
	seedProgressTTY  seedProgressMode = "tty"
	seedProgressLog  seedProgressMode = "log"
	seedProgressOff  seedProgressMode = "off"
)

type seedProgressReporter struct {
	mu       sync.Mutex
	mode     seedProgressMode
	out      io.Writer
	interval time.Duration
	nextEmit time.Time
	start    time.Time
}

func newSeedProgressReporter(out io.Writer, raw string) (*seedProgressReporter, error) {
	requested, err := parseSeedProgressMode(raw)
	if err != nil {
		return nil, err
	}
	mode := requested
	if requested == seedProgressAuto {
		if isTTYWriter(out) {
			mode = seedProgressTTY
		} else {
			mode = seedProgressLog
		}
	}
	if requested == seedProgressTTY && !isTTYWriter(out) {
		_, _ = fmt.Fprintln(out, "TTY progress requested but stdout is not a terminal; falling back to log progress")
		mode = seedProgressLog
	}

	interval := 2 * time.Second
	if mode == seedProgressTTY {
		interval = 250 * time.Millisecond
	}

	return &seedProgressReporter{
		mode:     mode,
		out:      out,
		interval: interval,
		nextEmit: time.Now(),
		start:    time.Now(),
	}, nil
}

func (r *seedProgressReporter) Emit(snapshot seeding.ProgressSnapshot) {
	if r == nil || r.mode == seedProgressOff {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Before(r.nextEmit) {
		return
	}
	r.nextEmit = now.Add(r.interval)

	elapsed := time.Since(r.start).Round(time.Second)
	line := fmt.Sprintf(
		"seed phase=%s namespaces=%d/%d mounts=%d/%d secrets=%d/%d elapsed=%s",
		snapshot.Phase,
		snapshot.NamespacesCreated,
		snapshot.PlannedNamespaces,
		snapshot.MountsCreated,
		snapshot.PlannedMounts,
		snapshot.SecretsWritten,
		snapshot.TotalSecrets,
		elapsed,
	)

	if r.mode == seedProgressTTY {
		_, _ = fmt.Fprintf(r.out, "\r%s", line)
		return
	}
	_, _ = fmt.Fprintln(r.out, line)
}

func (r *seedProgressReporter) Close() {
	if r == nil || r.mode != seedProgressTTY {
		return
	}
	_, _ = fmt.Fprintln(r.out)
}

func parseSeedProgressMode(raw string) (seedProgressMode, error) {
	mode := seedProgressMode(strings.TrimSpace(strings.ToLower(raw)))
	switch mode {
	case seedProgressAuto, seedProgressTTY, seedProgressLog, seedProgressOff:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid progress mode %q: must be one of auto, tty, log, off", raw)
	}
}

func isTTYWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}
