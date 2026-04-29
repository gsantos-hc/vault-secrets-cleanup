package deletion

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	gproto "google.golang.org/protobuf/proto"
)

type ActionExecutor interface {
	Delete(ctx context.Context, action *vpb.SecretAction) error
}

type Engine struct {
	executor       ActionExecutor
	rateLimiter    *ratelimit.Limiter
	circuitBreaker *CircuitBreaker
	workers        int
	progress       *ProgressTracker
	onProgress     func(ProgressSnapshot)
	planFile       string
	dryRun         bool
}

type ProgressSnapshot struct {
	Completed  int
	Failed     int
	Total      int
	Percentage float64
	ETA        time.Duration
}

type Config struct {
	Executor       ActionExecutor
	Deleter        SecretDeleter
	RateLimiter    *ratelimit.Limiter
	RetryConfig    retry.Config
	CircuitBreaker *CircuitBreaker
	Workers        int
	DryRun         bool
	PlanFile       string
	OnProgress     func(ProgressSnapshot)
}

func NewEngine(config Config) *Engine {
	executor := config.Executor
	if executor == nil {
		executor = NewExecutor(ExecutorConfig{
			Deleter:     config.Deleter,
			RetryConfig: config.RetryConfig,
			DryRun:      config.DryRun,
		})
	}

	cb := config.CircuitBreaker
	if cb == nil {
		cb = NewCircuitBreaker(10)
	}

	return &Engine{
		executor:       executor,
		rateLimiter:    config.RateLimiter,
		circuitBreaker: cb,
		workers:        defaultWorkers(config.Workers),
		onProgress:     config.OnProgress,
		planFile:       config.PlanFile,
		dryRun:         config.DryRun,
	}
}

func (e *Engine) Execute(ctx context.Context, plan *vpb.DeletionPlan) error {
	if plan == nil {
		return fmt.Errorf("plan is required")
	}
	if e.executor == nil {
		return fmt.Errorf("executor is required")
	}

	pendingCount := e.countPendingActions(plan)
	e.progress = NewProgressTracker(pendingCount)
	if pendingCount == 0 {
		return nil
	}

	pendingActions := e.pendingActions(plan)
	workers := defaultWorkers(e.workers)

	type deletionResult struct {
		action *vpb.SecretAction
		err    error
	}

	jobs := make(chan *vpb.SecretAction)
	results := make(chan deletionResult)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for action := range jobs {
				if err := ctx.Err(); err != nil {
					results <- deletionResult{action: action, err: err}
					continue
				}

				if e.rateLimiter != nil {
					if err := e.rateLimiter.Wait(ctx); err != nil {
						results <- deletionResult{action: action, err: err}
						continue
					}
				}

				err := e.executor.Delete(ctx, action)
				results <- deletionResult{action: action, err: err}
			}
		}()
	}

	next := 0
	inFlight := 0
	stopDispatch := false
	var ctxErr error
	var circuitErr error

	for next < len(pendingActions) || inFlight > 0 {
		for !stopDispatch && inFlight < workers && next < len(pendingActions) {
			if err := ctx.Err(); err != nil {
				ctxErr = err
				stopDispatch = true
				break
			}
			if !e.circuitBreaker.Allow() {
				if circuitErr == nil {
					circuitErr = e.circuitBreaker.Error()
				}
				stopDispatch = true
				break
			}

			jobs <- pendingActions[next]
			next++
			inFlight++
		}

		if inFlight == 0 {
			break
		}

		result := <-results
		inFlight--

		if result.err != nil {
			if errors.Is(result.err, context.Canceled) || errors.Is(result.err, context.DeadlineExceeded) {
				if ctxErr == nil {
					ctxErr = result.err
				}
				stopDispatch = true
				continue
			}

			e.circuitBreaker.RecordFailure(result.err)
			e.progress.RecordFailure()
			result.action.Status = "failed"
			result.action.Error = result.err.Error()
			result.action.DeletedAt = ""
		} else {
			if circuitErr == nil {
				e.circuitBreaker.RecordSuccess()
			}
			e.progress.RecordSuccess()
			if !e.dryRun {
				result.action.Status = "deleted"
				result.action.DeletedAt = time.Now().UTC().Format(time.RFC3339)
				result.action.Error = ""
			}
		}

		if e.circuitBreaker.IsOpen() {
			if circuitErr == nil {
				circuitErr = e.circuitBreaker.Error()
			}
			stopDispatch = true
		}

		if !e.dryRun && e.planFile != "" {
			if err := e.savePlan(plan); err != nil {
				close(jobs)
				wg.Wait()
				return err
			}
		}

		e.emitProgress()
	}

	close(jobs)
	wg.Wait()

	if ctxErr != nil {
		return ctxErr
	}
	if circuitErr != nil {
		return circuitErr
	}

	return nil
}

func (e *Engine) emitProgress() {
	if e.onProgress == nil || e.progress == nil {
		return
	}
	completed, failed, total, percentage := e.progress.GetProgress()
	e.onProgress(ProgressSnapshot{
		Completed:  completed,
		Failed:     failed,
		Total:      total,
		Percentage: percentage,
		ETA:        e.progress.GetETA(),
	})
}

func (e *Engine) shouldDelete(action *vpb.SecretAction, cfg *vpb.PlanConfig) bool {
	if action == nil {
		return false
	}

	switch action.GetCategory() {
	case "stale":
		return true
	case "unknown":
		return cfg != nil && cfg.GetIncludeUnknown()
	default:
		return false
	}
}

func (e *Engine) countPendingActions(plan *vpb.DeletionPlan) int {
	count := 0
	for _, action := range plan.Actions {
		if action.GetStatus() == "deleted" {
			continue
		}
		if e.shouldDelete(action, plan.GetConfig()) {
			count++
		}
	}
	return count
}

func (e *Engine) pendingActions(plan *vpb.DeletionPlan) []*vpb.SecretAction {
	actions := make([]*vpb.SecretAction, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		if action.GetStatus() == "deleted" {
			continue
		}
		if e.shouldDelete(action, plan.GetConfig()) {
			actions = append(actions, action)
		}
	}
	return actions
}

func defaultWorkers(workers int) int {
	if workers <= 0 {
		return 10
	}
	return workers
}

func (e *Engine) savePlan(plan *vpb.DeletionPlan) error {
	blob, err := gproto.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}

	tmpPath := e.planFile + ".tmp"
	if err := os.WriteFile(tmpPath, blob, 0o644); err != nil {
		return fmt.Errorf("write temp plan: %w", err)
	}
	if err := os.Rename(tmpPath, e.planFile); err != nil {
		return fmt.Errorf("rename temp plan: %w", err)
	}
	return nil
}
