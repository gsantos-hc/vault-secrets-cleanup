package seeding

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
)

type VaultWriter interface {
	CreateNamespace(ctx context.Context, namespace string) error
	EnableKVMount(ctx context.Context, namespace, mountPath string, kvVersion int) error
	WriteKVSecret(ctx context.Context, namespace, mountPath, secretPath string, data map[string]any, kvVersion int) error
}

type Waiter interface {
	Wait(ctx context.Context) error
}

type Config struct {
	Writer          VaultWriter
	RateLimiter     Waiter
	Retry           retry.Config
	NamespaceCount  int
	TotalSecrets    int
	KV2Probability  float64
	RandomSeed      int64
	NamespacePrefix string
	MountPrefix     string
	DryRun          bool
}

type Result struct {
	PlannedNamespaces int
	PlannedMounts     int
	PlannedSecrets    int
	NamespacesCreated int
	MountsCreated     int
	SecretsWritten    int
}

type Engine struct {
	cfg Config
}

func NewEngine(cfg Config) *Engine {
	return &Engine{cfg: cfg}
}

func (e *Engine) Run(ctx context.Context) (Result, error) {
	if e.cfg.Writer == nil {
		return Result{}, fmt.Errorf("writer is required")
	}
	if e.cfg.RateLimiter == nil {
		return Result{}, fmt.Errorf("rate limiter is required")
	}

	rng := rand.New(rand.NewSource(e.cfg.RandomSeed))
	plan, err := Allocate(AllocationInput{
		NamespaceCount: e.cfg.NamespaceCount,
		TotalSecrets:   e.cfg.TotalSecrets,
		KV2Probability: e.cfg.KV2Probability,
		RNG:            rng,
	})
	if err != nil {
		return Result{}, err
	}

	res := Result{PlannedNamespaces: len(plan), PlannedSecrets: e.cfg.TotalSecrets}
	for i := range plan {
		res.PlannedMounts += len(plan[i].Mounts)
	}

	if e.cfg.DryRun {
		return res, nil
	}

	for _, ns := range plan {
		nsName := withPrefix(e.cfg.NamespacePrefix, ns.Name)

		if err := e.wait(ctx); err != nil {
			return res, err
		}
		if err := retry.DoWithRetryable(ctx, e.cfg.Retry, func() error {
			return e.cfg.Writer.CreateNamespace(ctx, nsName)
		}, retry.IsRetryable); err != nil {
			return res, err
		}
		res.NamespacesCreated++

		for _, mount := range ns.Mounts {
			mountName := withPrefix(e.cfg.MountPrefix, mount.Name)
			if err := e.wait(ctx); err != nil {
				return res, err
			}
			if err := retry.DoWithRetryable(ctx, e.cfg.Retry, func() error {
				return e.cfg.Writer.EnableKVMount(ctx, nsName, mountName, mount.KVVersion)
			}, retry.IsRetryable); err != nil {
				return res, err
			}
			res.MountsCreated++

			for i := 0; i < mount.SecretCount; i++ {
				secretName := fmt.Sprintf("secret-%06d", i+1)
				if err := e.wait(ctx); err != nil {
					return res, err
				}
				if err := retry.DoWithRetryable(ctx, e.cfg.Retry, func() error {
					return e.cfg.Writer.WriteKVSecret(ctx, nsName, mountName, secretName, map[string]any{"value": rng.Int63()}, mount.KVVersion)
				}, retry.IsRetryable); err != nil {
					return res, err
				}
				res.SecretsWritten++
			}
		}
	}

	return res, nil
}

func (e *Engine) wait(ctx context.Context) error {
	if e.cfg.RateLimiter == nil {
		return nil
	}
	return e.cfg.RateLimiter.Wait(ctx)
}

func withPrefix(prefix, name string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return name
	}
	return fmt.Sprintf("%s-%s", prefix, name)
}
