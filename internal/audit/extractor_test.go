package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractorExtract(t *testing.T) {
	t.Run("extracts kv v2 read", func(t *testing.T) {
		event := AuditEvent{
			Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Type: "response",
			Request: RequestInfo{
				Operation:     "read",
				Path:          "secret/data/myapp/config",
				MountAccessor: "kv_12345",
				MountType:     "kv",
				Namespace: NamespaceInfo{
					ID:   "ns123",
					Path: "prod",
				},
			},
		}

		extractor := NewExtractor()
		record := extractor.Extract(event)

		require.NotNil(t, record)
		assert.Equal(t, "prod", record.NamespacePath)
		assert.Equal(t, "secret/", record.MountPath)
		assert.Equal(t, "myapp/config", record.SecretPath)
		assert.Equal(t, "read", record.AccessType)
		assert.EqualValues(t, 1, record.AccessCount)
	})

	t.Run("maps create and update to write", func(t *testing.T) {
		event := AuditEvent{
			Time: time.Now(),
			Type: "response",
			Request: RequestInfo{
				Operation: "update",
				Path:      "secret/data/myapp/config",
				MountType: "kv",
			},
		}

		record := NewExtractor().Extract(event)
		require.NotNil(t, record)
		assert.Equal(t, "write", record.AccessType)
	})

	t.Run("strips kv v2 metadata api segment", func(t *testing.T) {
		event := AuditEvent{
			Time: time.Now(),
			Type: "response",
			Request: RequestInfo{
				Operation: "read",
				Path:      "secret/metadata/myapp/config",
				MountType: "kv",
			},
		}

		record := NewExtractor().Extract(event)
		require.NotNil(t, record)
		assert.Equal(t, "secret/", record.MountPath)
		assert.Equal(t, "myapp/config", record.SecretPath)
	})

	t.Run("strips kv v2 subkeys api segment", func(t *testing.T) {
		event := AuditEvent{
			Time: time.Now(),
			Type: "response",
			Request: RequestInfo{
				Operation: "read",
				Path:      "secret/subkeys/myapp/config",
				MountType: "kv",
			},
		}

		record := NewExtractor().Extract(event)
		require.NotNil(t, record)
		assert.Equal(t, "secret/", record.MountPath)
		assert.Equal(t, "myapp/config", record.SecretPath)
	})

	t.Run("preserves secret segments named data", func(t *testing.T) {
		event := AuditEvent{
			Time: time.Now(),
			Type: "response",
			Request: RequestInfo{
				Operation: "read",
				Path:      "secret/data/data/value",
				MountType: "kv",
			},
		}

		record := NewExtractor().Extract(event)
		require.NotNil(t, record)
		assert.Equal(t, "secret/", record.MountPath)
		assert.Equal(t, "data/value", record.SecretPath)
	})

	t.Run("supports nested mount paths for kv v2 api segments", func(t *testing.T) {
		testCases := []struct {
			name       string
			path       string
			mountPath  string
			secretPath string
		}{
			{name: "data", path: "ops/kv/data/app/config", mountPath: "ops/kv/", secretPath: "app/config"},
			{name: "metadata", path: "ops/kv/metadata/app/config", mountPath: "ops/kv/", secretPath: "app/config"},
			{name: "subkeys", path: "ops/kv/subkeys/app/config", mountPath: "ops/kv/", secretPath: "app/config"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				event := AuditEvent{
					Time: time.Now(),
					Type: "response",
					Request: RequestInfo{
						Operation: "read",
						Path:      tc.path,
						MountType: "kv",
					},
				}

				record := NewExtractor().Extract(event)
				require.NotNil(t, record)
				assert.Equal(t, tc.mountPath, record.MountPath)
				assert.Equal(t, tc.secretPath, record.SecretPath)
			})
		}
	})

	t.Run("skips non kv events", func(t *testing.T) {
		event := AuditEvent{
			Type: "response",
			Request: RequestInfo{
				MountType: "pki",
				Path:      "pki/issue/example",
			},
		}

		record := NewExtractor().Extract(event)
		assert.Nil(t, record)
	})

	t.Run("skips non response events", func(t *testing.T) {
		event := AuditEvent{
			Type: "request",
			Request: RequestInfo{
				MountType: "kv",
				Path:      "secret/data/myapp/config",
			},
		}

		record := NewExtractor().Extract(event)
		assert.Nil(t, record)
	})
}
