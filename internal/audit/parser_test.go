package audit

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParserParse(t *testing.T) {
	t.Run("parses valid audit log lines", func(t *testing.T) {
		logData := `{"time":"2024-01-01T00:00:00Z","type":"response","request":{"path":"secret/data/app/config","operation":"read","mount_type":"kv"}}
{"time":"2024-01-01T00:01:00Z","type":"response","request":{"path":"secret/data/app/db","operation":"read","mount_type":"kv"}}`

		parser := NewParser()
		var events []AuditEvent

		err := parser.Parse(context.Background(), bytes.NewBufferString(logData), func(event AuditEvent) error {
			events = append(events, event)
			return nil
		})

		require.NoError(t, err)
		require.Len(t, events, 2)
		assert.Equal(t, "secret/data/app/config", events[0].Request.Path)
	})

	t.Run("skips malformed json lines", func(t *testing.T) {
		logData := `{"time":"2024-01-01T00:00:00Z","type":"response"
{"time":"2024-01-01T00:01:00Z","type":"response","request":{"path":"secret/data/app/db","operation":"read","mount_type":"kv"}}`

		parser := NewParser()
		var events []AuditEvent

		err := parser.Parse(context.Background(), bytes.NewBufferString(logData), func(event AuditEvent) error {
			events = append(events, event)
			return nil
		})

		require.NoError(t, err)
		require.Len(t, events, 1)
	})

	t.Run("stops on context cancellation", func(t *testing.T) {
		logData := `{"time":"2024-01-01T00:00:00Z","type":"response","request":{"path":"secret/data/app/config"}}`

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		parser := NewParser()
		err := parser.Parse(ctx, bytes.NewBufferString(logData), func(event AuditEvent) error {
			return nil
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})

	t.Run("parses RFC3339 timestamps", func(t *testing.T) {
		logData := `{"time":"2024-01-01T00:00:00Z","type":"response","request":{"path":"secret/data/app/config"}}`

		parser := NewParser()
		var got AuditEvent
		err := parser.Parse(context.Background(), bytes.NewBufferString(logData), func(event AuditEvent) error {
			got = event
			return nil
		})

		require.NoError(t, err)
		assert.True(t, got.Time.Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
	})
}
