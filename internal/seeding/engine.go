package seeding

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"

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
	Workers         int
	NamespacePrefix string
	MountPrefix     string
	DryRun          bool
	OnProgress      func(ProgressSnapshot)
}

type ProgressSnapshot struct {
	Phase             string
	NamespacesCreated int
	PlannedNamespaces int
	MountsCreated     int
	PlannedMounts     int
	SecretsWritten    int
	TotalSecrets      int
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

type mountWorkItem struct {
	namespace string
	path      string
	kvVersion int
	secrets   int
}

type secretWorkItem struct {
	namespace  string
	mountPath  string
	secretPath string
	data       map[string]any
	kvVersion  int
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
		e.emitProgress("complete", res)
		return res, nil
	}

	workers := defaultWorkers(e.cfg.Workers)
	mounts := make([]mountWorkItem, 0, res.PlannedMounts)
	secrets := make([]secretWorkItem, 0, res.PlannedSecrets)
	namespaces := make([]string, 0, len(plan))
	for _, ns := range plan {
		nsName := withPrefix(e.cfg.NamespacePrefix, ns.Name)
		namespaces = append(namespaces, nsName)

		for _, mount := range ns.Mounts {
			mountName := withPrefix(e.cfg.MountPrefix, mount.Name)
			mounts = append(mounts, mountWorkItem{
				namespace: nsName,
				path:      mountName,
				kvVersion: mount.KVVersion,
				secrets:   mount.SecretCount,
			})
			for i := 0; i < mount.SecretCount; i++ {
				secrets = append(secrets, secretWorkItem{
					namespace:  nsName,
					mountPath:  mountName,
					secretPath: fmt.Sprintf("secret-%06d", i+1),
					data:       map[string]any{"value": rng.Int63()},
					kvVersion:  mount.KVVersion,
				})
			}
		}
	}

	if err := runStage(ctx, namespaces, workers, func(ctx context.Context, namespace string) error {
		return e.executeWrite(ctx, func() error {
			return e.cfg.Writer.CreateNamespace(ctx, namespace)
		})
	}, func() {
		res.NamespacesCreated++
		e.emitProgress("namespaces", res)
	}); err != nil {
		return res, err
	}

	if err := runStage(ctx, mounts, workers, func(ctx context.Context, mount mountWorkItem) error {
		return e.executeWrite(ctx, func() error {
			return e.cfg.Writer.EnableKVMount(ctx, mount.namespace, mount.path, mount.kvVersion)
		})
	}, func() {
		res.MountsCreated++
		e.emitProgress("mounts", res)
	}); err != nil {
		return res, err
	}

	if err := runStage(ctx, secrets, workers, func(ctx context.Context, secret secretWorkItem) error {
		return e.executeWrite(ctx, func() error {
			return e.cfg.Writer.WriteKVSecret(ctx, secret.namespace, secret.mountPath, secret.secretPath, secret.data, secret.kvVersion)
		})
	}, func() {
		res.SecretsWritten++
		e.emitProgress("secrets", res)
	}); err != nil {
		return res, err
	}

	e.emitProgress("complete", res)
	return res, nil
}

func (e *Engine) executeWrite(ctx context.Context, op func() error) error {
	if err := e.wait(ctx); err != nil {
		return err
	}
	return retry.DoWithRetryable(ctx, e.cfg.Retry, op, retry.IsRetryable)
}

func runStage[T any](ctx context.Context, items []T, workers int, run func(context.Context, T) error, onSuccess func()) error {
	if len(items) == 0 {
		return nil
	}

	workers = min(workers, len(items))
	jobs := make(chan T)
	results := make(chan error)
	stageCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if err := stageCtx.Err(); err != nil {
					results <- err
					continue
				}
				results <- run(stageCtx, item)
			}
		}()
	}

	next := 0
	inFlight := 0
	stopDispatch := false
	var firstErr error

	for next < len(items) || inFlight > 0 {
		for !stopDispatch && inFlight < workers && next < len(items) {
			if err := stageCtx.Err(); err != nil {
				firstErr = err
				stopDispatch = true
				break
			}
			jobs <- items[next]
			next++
			inFlight++
		}

		if inFlight == 0 {
			break
		}

		err := <-results
		inFlight--
		if err != nil {
			if firstErr == nil {
				firstErr = err
				stopDispatch = true
				cancel()
			}
			continue
		}
		onSuccess()
	}

	close(jobs)
	wg.Wait()

	if firstErr != nil {
		return firstErr
	}
	return nil
}

func (e *Engine) emitProgress(phase string, res Result) {
	if e.cfg.OnProgress == nil {
		return
	}
	e.cfg.OnProgress(ProgressSnapshot{
		Phase:             phase,
		NamespacesCreated: res.NamespacesCreated,
		PlannedNamespaces: res.PlannedNamespaces,
		MountsCreated:     res.MountsCreated,
		PlannedMounts:     res.PlannedMounts,
		SecretsWritten:    res.SecretsWritten,
		TotalSecrets:      res.PlannedSecrets,
	})
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

func defaultWorkers(workers int) int {
	if workers <= 0 {
		return 1
	}
	return workers
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
