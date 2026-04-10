package audit

import (
	"sync"
	"time"

	vpb "github.com/gsantos-hc/vault-secrets-cleanup/pkg/proto"
)

type Aggregator struct {
	mu      sync.Mutex
	records map[string]*vpb.AccessRecord
}

func NewAggregator() *Aggregator {
	return &Aggregator{records: make(map[string]*vpb.AccessRecord)}
}

func (a *Aggregator) Add(record *vpb.AccessRecord) {
	if record == nil {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	key := makeKey(record)
	existing, exists := a.records[key]
	if !exists {
		a.records[key] = record
		return
	}

	existingTime, existingErr := time.Parse(time.RFC3339, existing.LastAccessed)
	newTime, newErr := time.Parse(time.RFC3339, record.LastAccessed)
	if existingErr == nil && newErr == nil && newTime.After(existingTime) {
		existing.LastAccessed = record.LastAccessed
		existing.AccessType = record.AccessType
	}
	existing.AccessCount += record.AccessCount
}

func (a *Aggregator) GetRecords() []*vpb.AccessRecord {
	a.mu.Lock()
	defer a.mu.Unlock()

	out := make([]*vpb.AccessRecord, 0, len(a.records))
	for _, rec := range a.records {
		out = append(out, rec)
	}
	return out
}

func (a *Aggregator) Count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.records)
}

func makeKey(record *vpb.AccessRecord) string {
	return record.NamespaceId + "|" + record.MountAccessor + "|" + record.SecretPath
}
