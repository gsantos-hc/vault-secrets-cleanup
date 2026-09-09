// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/reporting"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/spf13/cobra"
	gproto "google.golang.org/protobuf/proto"
)

type reportOptions struct {
	planPath string
	format   string
	output   string
}

func newReportCmd() *cobra.Command {
	opts := reportOptions{}
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Generate a human-readable report from a deletion plan",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReport(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.planPath, "plan", "deletion-plan.pb", "Deletion plan protobuf file")
	cmd.Flags().StringVar(&opts.format, "format", "markdown", "Output format (markdown, json, csv)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "", "Output file path (stdout when omitted)")

	return cmd
}

func runReport(ctx context.Context, opts reportOptions) error {
	_ = ctx
	data, err := os.ReadFile(opts.planPath)
	if err != nil {
		return fmt.Errorf("failed to read plan: %w", err)
	}

	plan := &vpb.DeletionPlan{}
	if err := gproto.Unmarshal(data, plan); err != nil {
		return fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	report, err := reporting.NewGenerator().Generate(plan, opts.format)
	if err != nil {
		return fmt.Errorf("failed to generate report: %w", err)
	}

	if opts.output == "" {
		fmt.Print(report)
		return nil
	}

	if err := os.WriteFile(opts.output, []byte(report), 0o644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}
