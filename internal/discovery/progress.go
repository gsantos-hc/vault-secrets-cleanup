// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package discovery

import (
	"fmt"
	"sync"
	"time"
)

type ProgressTracker struct {
	mu         sync.Mutex
	startTime  time.Time
	namespaces int
	mounts     int
	secrets    int
	errors     []string
	running    bool
}

type ProgressSnapshot struct {
	Namespaces int
	Mounts     int
	Secrets    int
	Errors     int
	Elapsed    time.Duration
}

func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{errors: []string{}}
}

func (p *ProgressTracker) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.startTime = time.Now()
	p.running = true
}

func (p *ProgressTracker) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.running = false
}

func (p *ProgressTracker) AddNamespace() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.namespaces++
}

func (p *ProgressTracker) AddMount() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mounts++
}

func (p *ProgressTracker) AddSecrets(count int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.secrets += count
}

func (p *ProgressTracker) LogError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.errors = append(p.errors, err)
}

func (p *ProgressTracker) Summary() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Since(p.startTime)
	return fmt.Sprintf(
		"Discovery complete in %v\n  Namespaces: %d\n  Mounts: %d\n  Secrets: %d\n  Errors: %d\n",
		elapsed.Round(time.Second),
		p.namespaces,
		p.mounts,
		p.secrets,
		len(p.errors),
	)
}

func (p *ProgressTracker) Snapshot() ProgressSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()

	elapsed := time.Duration(0)
	if !p.startTime.IsZero() {
		elapsed = time.Since(p.startTime)
	}

	return ProgressSnapshot{
		Namespaces: p.namespaces,
		Mounts:     p.mounts,
		Secrets:    p.secrets,
		Errors:     len(p.errors),
		Elapsed:    elapsed,
	}
}
