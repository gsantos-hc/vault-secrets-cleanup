// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
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

	reporter, err := newProgressReporter(os.Stdout, "import")
	if err != nil {
		return err
	}
	defer reporter.Close()

	data, err := os.ReadFile(opts.input)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}
	reporter.Emit("read", 1, 1, 0, 0)

	records, err := parseImportedRecords(data, func(completed, total int) {
		reporter.Emit("aggregate", completed, total, 0, 0)
	})
	if err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}
	reporter.Emit("parsed", len(records), len(records), 0, 0)

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

type importedRecord struct {
	NamespacePath string `json:"namespace_path"`
	MountPath     string `json:"mount_path"`
	MountAccessor string `json:"mount_accessor"`
	SecretPath    string `json:"secret_path"`
	LastAccessed  string `json:"last_accessed"`
	Timestamp     string `json:"timestamp"`
	AccessType    string `json:"access_type"`
	AccessCount   int32  `json:"access_count"`
}

func parseImportedRecords(data []byte, onProgress func(completed, total int)) ([]*vpb.AccessRecord, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("input is empty")
	}

	records := make([]importedRecord, 0)
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &records); err != nil {
			return nil, err
		}
	} else {
		decoder := json.NewDecoder(bytes.NewReader(trimmed))
		for {
			var rec importedRecord
			err := decoder.Decode(&rec)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return nil, err
			}
			records = append(records, rec)
		}
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no records found")
	}

	return aggregateImportedRecords(records, onProgress)
}

func aggregateImportedRecords(imported []importedRecord, onProgress func(completed, total int)) ([]*vpb.AccessRecord, error) {
	aggregated := make(map[string]*vpb.AccessRecord, len(imported))
	latestByKey := make(map[string]time.Time, len(imported))

	for i, rec := range imported {
		normalized, ts, err := normalizeImportedRecord(rec)
		if err != nil {
			return nil, fmt.Errorf("record %d: %w", i, err)
		}

		key := normalized.MountAccessor + "|" + normalized.SecretPath
		existing, ok := aggregated[key]
		if !ok {
			aggregated[key] = normalized
			latestByKey[key] = ts
			if onProgress != nil {
				onProgress(i+1, len(imported))
			}
			continue
		}

		existing.AccessCount += normalized.AccessCount
		if ts.After(latestByKey[key]) {
			existing.LastAccessed = normalized.LastAccessed
			existing.AccessType = normalized.AccessType
			latestByKey[key] = ts
		}
		if existing.MountAccessor == "" && normalized.MountAccessor != "" {
			existing.MountAccessor = normalized.MountAccessor
		}

		if onProgress != nil {
			onProgress(i+1, len(imported))
		}
	}

	out := make([]*vpb.AccessRecord, 0, len(aggregated))
	for _, rec := range aggregated {
		out = append(out, rec)
	}
	return out, nil
}

func normalizeImportedRecord(rec importedRecord) (*vpb.AccessRecord, time.Time, error) {
	namespacePath := normalizeNamespacePath(rec.NamespacePath)
	mountPath := normalizeMountPath(namespacePath, rec.MountPath)
	if mountPath == "" {
		return nil, time.Time{}, fmt.Errorf("mount_path is required")
	}

	secretPath, ok := normalizeSecretPath(mountPath, rec.SecretPath)
	if !ok {
		return nil, time.Time{}, fmt.Errorf("invalid secret_path %q", rec.SecretPath)
	}

	timestampRaw := strings.TrimSpace(rec.LastAccessed)
	if timestampRaw == "" {
		timestampRaw = strings.TrimSpace(rec.Timestamp)
	}
	if timestampRaw == "" {
		return nil, time.Time{}, fmt.Errorf("timestamp is required")
	}

	ts, err := time.Parse(time.RFC3339, timestampRaw)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("invalid timestamp %q", timestampRaw)
	}

	accessType := strings.TrimSpace(rec.AccessType)
	if accessType == "" {
		accessType = "read"
	}
	accessCount := rec.AccessCount
	if accessCount <= 0 {
		accessCount = 1
	}

	return &vpb.AccessRecord{
		NamespacePath: namespacePath,
		MountPath:     mountPath,
		MountAccessor: strings.TrimSpace(rec.MountAccessor),
		SecretPath:    secretPath,
		LastAccessed:  ts.UTC().Format(time.RFC3339),
		AccessType:    accessType,
		AccessCount:   accessCount,
	}, ts.UTC(), nil
}

func normalizeNamespacePath(namespacePath string) string {
	ns := strings.Trim(strings.TrimSpace(namespacePath), "/")
	if ns == "" {
		return ""
	}
	return ns + "/"
}

func normalizeMountPath(namespacePath, mountPath string) string {
	mount := strings.Trim(strings.TrimSpace(mountPath), "/")
	if mount == "" {
		return ""
	}

	ns := strings.Trim(normalizeNamespacePath(namespacePath), "/")
	if ns != "" {
		prefix := ns + "/"
		mount = strings.TrimPrefix(mount, prefix)
	}

	if mount == "" {
		return ""
	}
	return mount + "/"
}

func normalizeSecretPath(mountPath, secretPath string) (string, bool) {
	secret := strings.Trim(strings.TrimSpace(secretPath), "/")
	if secret == "" {
		return "", false
	}

	mount := strings.Trim(mountPath, "/")
	if mount != "" {
		prefix := mount + "/"
		secret = strings.TrimPrefix(secret, prefix)
	}

	parts := strings.Split(secret, "/")
	if len(parts) == 0 {
		return "", false
	}

	if apiIdx := kvV2APISegmentIndex(parts); apiIdx >= 0 {
		if apiIdx+1 >= len(parts) {
			return "", false
		}
		secret = strings.Join(parts[apiIdx+1:], "/")
	}

	secret = strings.Trim(secret, "/")
	if secret == "" {
		return "", false
	}

	return secret, true
}

func kvV2APISegmentIndex(parts []string) int {
	for i, part := range parts {
		switch part {
		case "data", "metadata", "subkeys":
			return i
		}
	}
	return -1
}
