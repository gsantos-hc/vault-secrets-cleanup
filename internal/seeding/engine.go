// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package seeding

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
)

// ErrFailureThresholdExceeded is returned by Run when the number of failed
// individual operations reaches the configured MaxFailures limit.
var ErrFailureThresholdExceeded = errors.New("failure threshold exceeded")

// failureThresholdError wraps both the sentinel and the last triggering error
// so callers can use errors.Is against either.
type failureThresholdError struct {
	max  int
	got  int
	last error
}

func (e *failureThresholdError) Error() string {
	return fmt.Sprintf("stopped after %d failed operations (max %d): %v", e.got, e.max, e.last)
}

func (e *failureThresholdError) Is(target error) bool {
	return target == ErrFailureThresholdExceeded
}

func (e *failureThresholdError) Unwrap() error { return e.last }

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
	SkipNamespaces  bool
	SkipMounts      bool
	// MaxFailures controls how many individual operation failures are tolerated
	// before the engine stops. 0 = stop on first failure (default). A positive
	// value N stops execution after the Nth failure. -1 = never stop for
	// failures (all failures are recorded but execution continues).
	MaxFailures int
	DryRun      bool
	OnProgress  func(ProgressSnapshot)
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
	Failures          int
}

type Engine struct {
	cfg Config
}

type mountWorkItem struct {
	namespace string
	path      string
	kvVersion int
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

	namespaces := make([]string, 0, len(plan))
	mounts := make([]mountWorkItem, 0, res.PlannedMounts)
	for _, ns := range plan {
		nsName := withPrefix(e.cfg.NamespacePrefix, ns.Name)
		namespaces = append(namespaces, nsName)
		for _, mount := range ns.Mounts {
			mounts = append(mounts, mountWorkItem{
				namespace: nsName,
				path:      withPrefix(e.cfg.MountPrefix, mount.Name),
				kvVersion: mount.KVVersion,
			})
		}
	}

	if !e.cfg.SkipNamespaces {
		if err := runStage(ctx, workers, sliceSend(namespaces), func(ctx context.Context, namespace string) error {
			return e.executeWrite(ctx, func() error {
				return e.cfg.Writer.CreateNamespace(ctx, namespace)
			})
		}, func() {
			res.NamespacesCreated++
			e.emitProgress("namespaces", res)
		}, &res.Failures, e.cfg.MaxFailures); err != nil {
			return res, err
		}
	}

	if !e.cfg.SkipMounts {
		if err := runStage(ctx, workers, sliceSend(mounts), func(ctx context.Context, mount mountWorkItem) error {
			return e.executeWrite(ctx, func() error {
				return e.cfg.Writer.EnableKVMount(ctx, mount.namespace, mount.path, mount.kvVersion)
			})
		}, func() {
			res.MountsCreated++
			e.emitProgress("mounts", res)
		}, &res.Failures, e.cfg.MaxFailures); err != nil {
			return res, err
		}
	}

	if err := runStage(ctx, workers, func(ctx context.Context, ch chan<- secretWorkItem) {
		// rng is only accessed here, after Allocate has completed, so there is no
		// concurrent access; no mutex is needed.
		for _, ns := range plan {
			nsName := withPrefix(e.cfg.NamespacePrefix, ns.Name)
			for _, mount := range ns.Mounts {
				mountName := withPrefix(e.cfg.MountPrefix, mount.Name)
				for i := 0; i < mount.SecretCount; i++ {
					item := secretWorkItem{
						namespace:  nsName,
						mountPath:  mountName,
						secretPath: fmt.Sprintf("secret-%06d", i+1),
						data:       map[string]any{"value": rng.Int63()},
						kvVersion:  mount.KVVersion,
					}
					select {
					case ch <- item:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}, func(ctx context.Context, secret secretWorkItem) error {
		return e.executeWrite(ctx, func() error {
			return e.cfg.Writer.WriteKVSecret(ctx, secret.namespace, secret.mountPath, secret.secretPath, secret.data, secret.kvVersion)
		})
	}, func() {
		res.SecretsWritten++
		e.emitProgress("secrets", res)
	}, &res.Failures, e.cfg.MaxFailures); err != nil {
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

// sliceSend returns a send function that feeds every element of items into ch,
// stopping early when ctx is cancelled.
func sliceSend[T any](items []T) func(context.Context, chan<- T) {
	return func(ctx context.Context, ch chan<- T) {
		for _, item := range items {
			select {
			case ch <- item:
			case <-ctx.Done():
				return
			}
		}
	}
}

// runStage runs all items produced by send through run in parallel using workers
// goroutines. send is called in its own goroutine and should write items to the
// supplied channel, respecting ctx for early cancellation. onSuccess is called
// (from the collector goroutine) for each item that completes without error.
//
// failures is a shared counter incremented on every failed item. maxFailures
// controls when execution stops: 0 = stop on first failure, N > 0 = stop when
// failures reaches N, -1 = never stop for failures.
func runStage[T any](ctx context.Context, workers int, send func(context.Context, chan<- T), run func(context.Context, T) error, onSuccess func(), failures *int, maxFailures int) error {
	stageCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Buffered channels decouple the producer from workers and workers from the
	// collector, eliminating the head-of-line blocking that could cause deadlocks
	// with unbuffered channels.
	jobs := make(chan T, workers)
	results := make(chan error, workers)

	// Producer goroutine – closes jobs when it is done or the context is cancelled.
	go func() {
		defer close(jobs)
		send(stageCtx, jobs)
	}()

	// Worker goroutines – drain jobs and forward results.
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range jobs {
				if stageCtx.Err() != nil {
					results <- stageCtx.Err()
					continue
				}
				results <- run(stageCtx, item)
			}
		}()
	}

	// Close results once every worker has finished so the collector loop below
	// terminates naturally.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collector – process results in the calling goroutine so that onSuccess is
	// never called concurrently.
	var thresholdErr error
	for err := range results {
		if err != nil {
			*failures++
			exceeded := maxFailures == 0 || (maxFailures > 0 && *failures >= maxFailures)
			if exceeded && thresholdErr == nil {
				thresholdErr = &failureThresholdError{max: maxFailures, got: *failures, last: err}
				cancel() // signal producer and workers to stop early
			}
			continue
		}
		onSuccess()
	}

if err := ctx.Err(); err != nil {
	return err
}
return thresholdErr
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
