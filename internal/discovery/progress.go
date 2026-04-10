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
