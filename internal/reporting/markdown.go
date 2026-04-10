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
	b.WriteString(fmt.Sprintf("- Total secrets: %d\n", plan.GetStats().GetTotalSecrets()))
	b.WriteString(fmt.Sprintf("- Stale: %d\n", plan.GetStats().GetStaleCount()))
	b.WriteString(fmt.Sprintf("- Unknown: %d\n", plan.GetStats().GetUnknownCount()))
	b.WriteString(fmt.Sprintf("- Active: %d\n", plan.GetStats().GetActiveCount()))
	b.WriteString(fmt.Sprintf("- Excluded: %d\n", plan.GetStats().GetExcludedCount()))
	b.WriteString(fmt.Sprintf("- To delete: %d\n\n", plan.GetStats().GetToDeleteCount()))
	b.WriteString("## Actions\n\n")
	b.WriteString("| Namespace | Mount | Secret | Category | Reason |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, action := range plan.Actions {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			action.GetNamespacePath(), action.GetMountPath(), action.GetSecretPath(), action.GetCategory(), action.GetReason()))
	}

	return b.String()
}
