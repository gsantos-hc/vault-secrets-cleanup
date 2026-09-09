// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package deletion

import (
	"fmt"
	"sync"
	"time"
)

type ProgressTracker struct {
	mu         sync.Mutex
	total      int
	completed  int
	failed     int
	startTime  time.Time
	lastUpdate time.Time
}

func NewProgressTracker(total int) *ProgressTracker {
	return &ProgressTracker{
		total:      total,
		startTime:  time.Now(),
		lastUpdate: time.Now(),
	}
}

func (p *ProgressTracker) RecordSuccess() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.completed++
	p.lastUpdate = time.Now()
}

func (p *ProgressTracker) RecordFailure() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.completed++
	p.failed++
	p.lastUpdate = time.Now()
}

func (p *ProgressTracker) GetProgress() (completed, failed, total int, percentage float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.total > 0 {
		percentage = float64(p.completed) / float64(p.total) * 100
	}

	return p.completed, p.failed, p.total, percentage
}

func (p *ProgressTracker) GetETA() time.Duration {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.completed == 0 || p.total <= p.completed {
		return 0
	}

	elapsed := time.Since(p.startTime)
	avg := elapsed / time.Duration(p.completed)
	remaining := p.total - p.completed

	return avg * time.Duration(remaining)
}

func (p *ProgressTracker) String() string {
	completed, failed, total, pct := p.GetProgress()
	eta := p.GetETA()

	return fmt.Sprintf("Progress: %d/%d (%.1f%%) | Failed: %d | ETA: %v", completed, total, pct, failed, eta.Round(time.Second))
}
