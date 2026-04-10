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
