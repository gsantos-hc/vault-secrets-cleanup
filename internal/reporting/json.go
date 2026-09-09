// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package reporting

import (
	"encoding/json"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

func formatJSON(plan *vpb.DeletionPlan) (string, error) {
	b, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
