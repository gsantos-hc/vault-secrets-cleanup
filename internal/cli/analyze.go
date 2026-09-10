// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gsantos-hc/vault-secrets-cleanup/internal/audit"
	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/spf13/cobra"
	gproto "google.golang.org/protobuf/proto"
)

type analyzeOptions struct {
	auditLog string
	output   string
}

func newAnalyzeCmd() *cobra.Command {
	opts := analyzeOptions{}
	cmd := &cobra.Command{
		Use:   "analyze",
		Short: "Analyze audit logs to extract secret access patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAnalyze(cmd.Context(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.auditLog, "audit-log", "", "Audit log file path or glob (required)")
	cmd.Flags().StringVarP(&opts.output, "output", "o", "access.pb", "Output file for access protobuf")
	_ = cmd.MarkFlagRequired("audit-log")
	return cmd
}

func runAnalyze(ctx context.Context, opts analyzeOptions) error {
	if opts.auditLog == "" {
		return fmt.Errorf("audit log path is required")
	}

	files, err := filepath.Glob(opts.auditLog)
	if err != nil {
		return fmt.Errorf("failed to glob audit logs: %w", err)
	}
	if len(files) == 0 {
		return fmt.Errorf("no audit log files found matching: %s", opts.auditLog)
	}

	reporter, err := newProgressReporter(os.Stdout, "analyze")
	if err != nil {
		return err
	}
	defer reporter.Close()

	parser := NewAuditParser()
	extractor := audit.NewExtractor()
	aggregator := audit.NewAggregator()
	events := 0

	for idx, file := range files {
		if err := processAuditFile(ctx, file, parser, extractor, aggregator, func() {
			events++
			reporter.Emit("parse", events, 0, 0, 0)
		}); err != nil {
			return err
		}
		reporter.Emit("files", idx+1, len(files), 0, 0)
	}

	records := aggregator.GetRecords()
	data := &vpb.AccessData{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Source:    "audit_log",
		Records:   records,
		Stats:     calculateAccessStats(records),
	}

	blob, err := gproto.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal access data: %w", err)
	}

	if err := os.WriteFile(opts.output, blob, 0o644); err != nil {
		return fmt.Errorf("failed to write access data: %w", err)
	}

	if logger := GetLogger(); logger != nil {
		logger.Info("analysis complete", "output", opts.output, "records", len(records))
	}
	return nil
}

type auditReader interface {
	Read([]byte) (int, error)
}

type parserAdapter struct {
	parser *audit.Parser
}

func NewAuditParser() *parserAdapter {
	return &parserAdapter{parser: audit.NewParser()}
}

func (p *parserAdapter) Parse(ctx context.Context, reader auditReader, handler func(event audit.AuditEvent) error) error {
	return p.parser.Parse(ctx, reader, handler)
}

func processAuditFile(ctx context.Context, path string, parser *parserAdapter, extractor *audit.Extractor, aggregator *audit.Aggregator, onEvent func()) error {
	reader, err := audit.OpenFile(path)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	return parser.Parse(ctx, reader, func(event audit.AuditEvent) error {
		if onEvent != nil {
			onEvent()
		}
		aggregator.Add(extractor.Extract(event))
		return nil
	})
}

func calculateAccessStats(records []*vpb.AccessRecord) *vpb.AccessStats {
	stats := &vpb.AccessStats{
		TotalRecords:  int32(len(records)),
		UniqueSecrets: int32(len(records)),
	}
	for _, record := range records {
		switch record.AccessType {
		case "read":
			stats.ReadCount++
		case "write":
			stats.WriteCount++
		case "delete":
			stats.DeleteCount++
		}
	}
	return stats
}
