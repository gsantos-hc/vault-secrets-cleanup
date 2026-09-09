// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package audit

import (
	"testing"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
	"github.com/stretchr/testify/assert"
)

func TestAggregatorAdd(t *testing.T) {
	agg := NewAggregator()

	agg.Add(&vpb.AccessRecord{
		NamespacePath: "team-a",
		MountAccessor: "kv_1",
		SecretPath:    "app/config",
		LastAccessed:  "2024-01-01T00:00:00Z",
		AccessType:    "read",
		AccessCount:   1,
	})
	agg.Add(&vpb.AccessRecord{
		NamespacePath: "team-a",
		MountAccessor: "kv_1",
		SecretPath:    "app/config",
		LastAccessed:  "2024-01-02T00:00:00Z",
		AccessType:    "write",
		AccessCount:   1,
	})

	records := agg.GetRecords()
	assert.Len(t, records, 1)
	assert.Equal(t, "2024-01-02T00:00:00Z", records[0].LastAccessed)
	assert.Equal(t, "write", records[0].AccessType)
	assert.EqualValues(t, 2, records[0].AccessCount)
}

func TestAggregatorCountUnique(t *testing.T) {
	agg := NewAggregator()
	agg.Add(&vpb.AccessRecord{NamespacePath: "team-a", MountAccessor: "a", SecretPath: "a"})
	agg.Add(&vpb.AccessRecord{NamespacePath: "team-a", MountAccessor: "a", SecretPath: "a"})
	agg.Add(&vpb.AccessRecord{NamespacePath: "team-a", MountAccessor: "a", SecretPath: "b"})

	assert.Equal(t, 2, agg.Count())
}
