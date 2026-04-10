package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/spf13/cobra"
	gproto "google.golang.org/protobuf/proto"
)

type importOptions struct {
	input  string
	output string
}

func newImportCmd() *cobra.Command {
	opts := importOptions{}
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import external access data",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImport(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVarP(&opts.input, "input", "i", "", "Input JSON file (required)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "access.pb", "Output file for access protobuf")
	_ = cmd.MarkFlagRequired("input")
	return cmd
}

func runImport(ctx context.Context, opts importOptions) error {
	_ = ctx
	if opts.input == "" {
		return fmt.Errorf("input path is required")
	}

	data, err := os.ReadFile(opts.input)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	var records []*vpb.AccessRecord
	if err := json.Unmarshal(data, &records); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	payload := &vpb.AccessData{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Source:    "external",
		Records:   records,
		Stats:     calculateAccessStats(records),
	}

	blob, err := gproto.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal access data: %w", err)
	}

	if err := os.WriteFile(opts.output, blob, 0o644); err != nil {
		return fmt.Errorf("failed to write access data: %w", err)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("import complete", "output", opts.output, "records", len(records))
	}
	return nil
}
