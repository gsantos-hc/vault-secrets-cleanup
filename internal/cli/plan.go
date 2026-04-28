package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/config"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/correlation"
	"github.com/gsantos-hc/vault-secrets-cleanup/internal/planning"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/spf13/cobra"
	gproto "google.golang.org/protobuf/proto"
)

type planOptions struct {
	inventory        string
	accessData       string
	output           string
	stalenessPeriod  string
	matchingStrategy string
	includeUnknown   bool
}

func newPlanCmd() *cobra.Command {
	opts := planOptions{}
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Generate deletion plan from inventory and access data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPlan(cmd.Context(), opts, GetConfig())
		},
	}

	cmd.Flags().StringVar(&opts.inventory, "inventory", "inventory.pb", "Inventory protobuf file")
	cmd.Flags().StringVar(&opts.accessData, "access-data", "access.pb", "Access protobuf file")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "deletion-plan.pb", "Output plan protobuf file")
	cmd.Flags().StringVar(&opts.stalenessPeriod, "staleness-period", "", "Override staleness threshold (examples: 5m, 24h, 365d)")
	cmd.Flags().StringVar(&opts.matchingStrategy, "matching-strategy", "accessor", "Matching strategy (accessor=mount accessor+secret path, path-based=namespace+mount+secret path)")
	cmd.Flags().BoolVar(&opts.includeUnknown, "include-unknown", false, "Include unknown-access secrets in deletion count")

	return cmd
}

func runPlan(ctx context.Context, opts planOptions, cfg *config.Config) error {
	_ = ctx
	if cfg == nil {
		return fmt.Errorf("configuration is required")
	}

	reporter, err := newProgressReporter(os.Stdout, "plan")
	if err != nil {
		return err
	}
	defer reporter.Close()

	inventoryBytes, err := os.ReadFile(opts.inventory)
	if err != nil {
		return fmt.Errorf("failed to read inventory: %w", err)
	}
	accessBytes, err := os.ReadFile(opts.accessData)
	if err != nil {
		return fmt.Errorf("failed to read access data: %w", err)
	}

	inventory := &vpb.Inventory{}
	if err := gproto.Unmarshal(inventoryBytes, inventory); err != nil {
		return fmt.Errorf("failed to unmarshal inventory: %w", err)
	}
	accessData := &vpb.AccessData{}
	if err := gproto.Unmarshal(accessBytes, accessData); err != nil {
		return fmt.Errorf("failed to unmarshal access data: %w", err)
	}

	strategy := correlation.MatchingStrategy(opts.matchingStrategy)
	if strategy != correlation.MatchingStrategyAccessor && strategy != correlation.MatchingStrategyPathBased {
		return fmt.Errorf("unknown matching strategy %q: must be one of accessor, path-based", opts.matchingStrategy)
	}
	engine := correlation.NewEngine(correlation.Config{
		MatchingStrategy: strategy,
		OnProgress: func(snapshot correlation.ProgressSnapshot) {
			reporter.Emit("correlation", snapshot.Completed, snapshot.Total, 0, 0)
		},
	})
	accessMap, err := engine.Correlate(inventory, accessData)
	if err != nil {
		return fmt.Errorf("correlation failed: %w", err)
	}

	stalenessPeriod := cfg.Staleness.DefaultPeriod
	if opts.stalenessPeriod != "" {
		stalenessPeriod = opts.stalenessPeriod
	}

	planner := planning.NewPlanner(planning.Config{
		StalenessConfig: planning.StalenessConfig{
			DefaultPeriod:     stalenessPeriod,
			NamespacePolicies: cfg.Staleness.Namespaces,
		},
		ExclusionConfig: planning.ExclusionConfig{
			NamespacePatterns: cfg.Exclusions.Namespaces,
			PathPatterns:      cfg.Exclusions.Paths,
			MountPatterns:     cfg.Exclusions.Mounts,
		},
		IncludeUnknown: opts.includeUnknown,
		OnProgress: func(snapshot planning.ProgressSnapshot) {
			reporter.Emit("planning", snapshot.Completed, snapshot.Total, 0, 0)
		},
	})

	planConfig := &vpb.PlanConfig{
		StalenessPeriod:   stalenessPeriod,
		MatchingStrategy:  string(strategy),
		IncludeUnknown:    opts.includeUnknown,
		ExclusionPatterns: append(append([]string{}, cfg.Exclusions.Namespaces...), append(cfg.Exclusions.Paths, cfg.Exclusions.Mounts...)...),
	}

	deletionPlan, err := planner.GeneratePlan(inventory, accessMap, planConfig)
	if err != nil {
		return fmt.Errorf("plan generation failed: %w", err)
	}
	deletionPlan.InventoryFile = opts.inventory
	deletionPlan.AccessFile = opts.accessData

	blob, err := gproto.Marshal(deletionPlan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}
	if err := os.WriteFile(opts.output, blob, 0o644); err != nil {
		return fmt.Errorf("failed to write plan: %w", err)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("plan generation complete", "output", opts.output, "to_delete", deletionPlan.GetStats().GetToDeleteCount())
	}

	return nil
}
