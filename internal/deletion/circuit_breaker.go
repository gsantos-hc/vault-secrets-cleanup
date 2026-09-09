// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package deletion

import (
	"fmt"
	"sync"
)

type CircuitBreaker struct {
	mu                  sync.Mutex
	maxFailures         int
	consecutiveFailures int
	isOpen              bool
	lastError           error
}

func NewCircuitBreaker(maxFailures int) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 1
	}

	return &CircuitBreaker{maxFailures: maxFailures}
}

func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return !cb.isOpen
}

func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures = 0
	cb.isOpen = false
	cb.lastError = nil
}

func (cb *CircuitBreaker) RecordFailure(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.consecutiveFailures++
	cb.lastError = err
	if cb.consecutiveFailures >= cb.maxFailures {
		cb.isOpen = true
	}
}

func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.isOpen
}

func (cb *CircuitBreaker) GetStats() (consecutiveFailures int, isOpen bool, lastError error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	return cb.consecutiveFailures, cb.isOpen, cb.lastError
}

func (cb *CircuitBreaker) Error() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if !cb.isOpen {
		return nil
	}

	return fmt.Errorf("circuit breaker open after %d consecutive failures (last error: %v)", cb.consecutiveFailures, cb.lastError)
}
