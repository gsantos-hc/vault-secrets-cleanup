package audit

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func BenchmarkParser_Parse(b *testing.B) {
	parser := NewParser()
	line := `{"time":"2026-01-01T00:00:00Z","type":"response","request":{"operation":"read","path":"secret/data/app/config","mount_type":"kv","mount_accessor":"kv_1","namespace":{"id":"ns1","path":"prod/app"}}}`
	payload := strings.Repeat(line+"\n", 1000)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		err := parser.Parse(context.Background(), strings.NewReader(payload), func(event AuditEvent) error {
			if event.Request.Path == "" {
				return fmt.Errorf("missing path")
			}
			return nil
		})
		if err != nil {
			b.Fatalf("parse failed: %v", err)
		}
	}
}
