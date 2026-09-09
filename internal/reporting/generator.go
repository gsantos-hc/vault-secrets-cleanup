// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package reporting

import (
	"fmt"
	"strings"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(plan *vpb.DeletionPlan, format string) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("plan is required")
	}

	switch strings.ToLower(format) {
	case "", "markdown", "md":
		return formatMarkdown(plan), nil
	case "json":
		return formatJSON(plan)
	case "csv":
		return formatCSV(plan)
	default:
		return "", fmt.Errorf("unsupported report format: %s", format)
	}
}
