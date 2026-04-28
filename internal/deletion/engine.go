package deletion

import (
	"context"
	"fmt"
	"os"
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

	for _, action := range plan.Actions {
		if !e.shouldDelete(action, plan.GetConfig()) {
			continue
		}
		if action.GetStatus() == "deleted" {
			continue
		}

		if !e.circuitBreaker.Allow() {
			return e.circuitBreaker.Error()
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if e.rateLimiter != nil {
			if err := e.rateLimiter.Wait(ctx); err != nil {
				return err
			}
		}

		err := e.executor.Delete(ctx, action)
		if err != nil {
			e.circuitBreaker.RecordFailure(err)
			e.progress.RecordFailure()
			action.Status = "failed"
			action.Error = err.Error()
			action.DeletedAt = ""
		} else {
			e.circuitBreaker.RecordSuccess()
			e.progress.RecordSuccess()
			if !e.dryRun {
				action.Status = "deleted"
				action.DeletedAt = time.Now().UTC().Format(time.RFC3339)
				action.Error = ""
			}
		}

		if !e.dryRun && e.planFile != "" {
			if err := e.savePlan(plan); err != nil {
				return err
			}
		}

		e.emitProgress()
	}

	if e.circuitBreaker.IsOpen() {
		return e.circuitBreaker.Error()
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
