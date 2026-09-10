// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package reporting

import (
	"fmt"
	"strings"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func formatMarkdown(plan *vpb.DeletionPlan) string {
	var b strings.Builder
	b.WriteString("# Deletion Plan Report\n\n")
	b.WriteString("## Summary\n\n")
	fmt.Fprintf(&b, "- Total secrets: %d\n", plan.GetStats().GetTotalSecrets())
	fmt.Fprintf(&b, "- Stale: %d\n", plan.GetStats().GetStaleCount())
	fmt.Fprintf(&b, "- Unknown: %d\n", plan.GetStats().GetUnknownCount())
	fmt.Fprintf(&b, "- Active: %d\n", plan.GetStats().GetActiveCount())
	fmt.Fprintf(&b, "- Excluded: %d\n", plan.GetStats().GetExcludedCount())
	fmt.Fprintf(&b, "- To delete: %d\n\n", plan.GetStats().GetToDeleteCount())
	b.WriteString("## Actions\n\n")
	b.WriteString("| Namespace | Mount | Secret | Category | Last Access | Reason |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, action := range plan.Actions {
		fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n",
			action.GetNamespacePath(), action.GetMountPath(), action.GetSecretPath(), action.GetCategory(), actionLastAccessAge(action), action.GetReason())
	}

	return b.String()
}
