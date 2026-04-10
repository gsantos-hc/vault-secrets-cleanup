package reporting

import (
	"bytes"
	"encoding/csv"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func formatCSV(plan *vpb.DeletionPlan) (string, error) {
	buf := bytes.NewBuffer(nil)
	w := csv.NewWriter(buf)
	if err := w.Write([]string{"namespace_path", "mount_path", "secret_path", "category", "last_access_age", "reason"}); err != nil {
		return "", err
	}

	for _, action := range plan.Actions {
		if err := w.Write([]string{action.GetNamespacePath(), action.GetMountPath(), action.GetSecretPath(), action.GetCategory(), actionLastAccessAge(action), action.GetReason()}); err != nil {
			return "", err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}

	return buf.String(), nil
}
