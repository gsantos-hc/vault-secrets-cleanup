package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/deletion"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/ratelimit"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/retry"
	vaultpkg "github.com/gsantos-hc/vault-secrets-cleanup/internal/vault"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/spf13/cobra"
	gproto "google.golang.org/protobuf/proto"
)

type executeOptions struct {
	plan           string
	rateLimit      float64
	circuitBreaker int
	dryRun         bool
	confirmAll     bool
}

type deletionEngine interface {
	Execute(ctx context.Context, plan *vpb.DeletionPlan) error
}

type executeVaultClient = *vaultpkg.Client

var createExecuteVaultClient = func(cfg *config.Config) (executeVaultClient, error) {
	return vaultpkg.NewClient(vaultpkg.Config{
		Address: cfg.Vault.Address,
		Token:   cfg.Vault.Token,
	})
}

var createExecuteEngine = func(client executeVaultClient, opts executeOptions, cfg *config.Config, onProgress func(deletion.ProgressSnapshot)) deletionEngine {
	rate := cfg.RateLimit.RequestsPerSecond
	if opts.rateLimit > 0 {
		rate = opts.rateLimit
	}
	threshold := opts.circuitBreaker
	if threshold <= 0 {
		threshold = 10
	}

	return deletion.NewEngine(deletion.Config{
		Deleter:        deletion.NewVaultSecretDeleter(client),
		RateLimiter:    ratelimit.New(rate),
		RetryConfig:    retry.DefaultConfig(),
		CircuitBreaker: deletion.NewCircuitBreaker(threshold),
		DryRun:         opts.dryRun,
		PlanFile:       opts.plan,
		OnProgress:     onProgress,
	})
}

func newExecuteCmd() *cobra.Command {
	opts := executeOptions{}
	cmd := &cobra.Command{
		Use:   "execute",
		Short: "Execute approved deletion plan",
		Long:  "Execute deletion plan with confirmation, circuit breaker protection, and resumable status updates.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecute(cmd.Context(), opts, GetConfig(), os.Stdin, cmd.OutOrStdout())
		},
	}

	cmd.Flags().StringVar(&opts.plan, "plan", "deletion-plan.pb", "Deletion plan protobuf file")
	cmd.Flags().Float64Var(&opts.rateLimit, "rate-limit", 0, "Override deletion requests per second")
	cmd.Flags().IntVar(&opts.circuitBreaker, "circuit-breaker", 10, "Max consecutive failures before stopping")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Simulate deletion without changing Vault")
	cmd.Flags().BoolVarP(&opts.confirmAll, "yes", "y", false, "Skip interactive confirmation")

	return cmd
}

func runExecute(ctx context.Context, opts executeOptions, cfg *config.Config, in io.Reader, out io.Writer) error {
	if cfg == nil {
		return fmt.Errorf("configuration is required")
	}

	reporter, err := newProgressReporter(out, "execute")
	if err != nil {
		return err
	}
	defer reporter.Close()

	blob, err := os.ReadFile(opts.plan)
	if err != nil {
		return fmt.Errorf("failed to read plan: %w", err)
	}

	plan := &vpb.DeletionPlan{}
	if err := gproto.Unmarshal(blob, plan); err != nil {
		return fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	printExecutionSummary(out, plan, opts)

	if !opts.dryRun && !opts.confirmAll {
		fmt.Fprint(out, "Type 'yes' to confirm deletion: ")
		reader := bufio.NewReader(in)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		if strings.TrimSpace(response) != "yes" {
			fmt.Fprintln(out, "Deletion cancelled")
			return nil
		}
	}

	client, err := createExecuteVaultClient(cfg)
	if err != nil {
		return fmt.Errorf("failed to create vault client: %w", err)
	}
	engine := createExecuteEngine(client, opts, cfg, func(snapshot deletion.ProgressSnapshot) {
		reporter.Emit("delete", snapshot.Completed, snapshot.Total, snapshot.Failed, snapshot.ETA)
	})

	if err := engine.Execute(ctx, plan); err != nil {
		if logger := GetLogger(); logger != nil {
			logger.Error("deletion failed", "error", err)
		}
		return fmt.Errorf("deletion failed: %w", err)
	}

	deleted, failed := countActionResults(plan)
	if logger := GetLogger(); logger != nil {
		logger.Info("deletion complete", "deleted", deleted, "failed", failed, "dry_run", opts.dryRun)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "=== Deletion Complete ===")
	if opts.dryRun {
		fmt.Fprintf(out, "Simulated deletions: %d\n", countWouldDelete(plan))
	} else {
		fmt.Fprintf(out, "Successfully deleted: %d\n", deleted)
	}
	fmt.Fprintf(out, "Failed: %d\n", failed)
	fmt.Fprintf(out, "Plan file: %s\n", opts.plan)

	if failed > 0 {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Some deletions failed. Re-run execute to retry failed items.")
	}

	return nil
}

func printExecutionSummary(out io.Writer, plan *vpb.DeletionPlan, opts executeOptions) {
	stats := plan.GetStats()
	fmt.Fprintln(out, "=== Deletion Plan Summary ===")
	fmt.Fprintf(out, "Total secrets: %d\n", stats.GetTotalSecrets())
	fmt.Fprintf(out, "To delete: %d\n", stats.GetToDeleteCount())
	fmt.Fprintf(out, "Stale: %d\n", stats.GetStaleCount())
	fmt.Fprintf(out, "Unknown: %d\n", stats.GetUnknownCount())
	fmt.Fprintln(out)

	if opts.dryRun {
		fmt.Fprintln(out, "DRY RUN MODE: no secrets will be deleted.")
	} else {
		fmt.Fprintln(out, "WARNING: This operation permanently deletes secrets from Vault.")
	}
	fmt.Fprintln(out)
}

func countActionResults(plan *vpb.DeletionPlan) (deleted int, failed int) {
	for _, action := range plan.GetActions() {
		switch action.GetStatus() {
		case "deleted":
			deleted++
		case "failed":
			failed++
		}
	}
	return deleted, failed
}

func countWouldDelete(plan *vpb.DeletionPlan) int {
	count := 0
	for _, action := range plan.GetActions() {
		if action.GetStatus() == "deleted" {
			continue
		}
		if action.GetCategory() == "stale" {
			count++
			continue
		}
		if action.GetCategory() == "unknown" && plan.GetConfig() != nil && plan.GetConfig().GetIncludeUnknown() {
			count++
		}
	}
	return count
}
