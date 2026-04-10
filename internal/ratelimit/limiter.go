package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
)

type Limiter struct {
	limiter *rate.Limiter
}

func New(requestsPerSecond float64) *Limiter {
	return NewWithBurst(requestsPerSecond, int(requestsPerSecond))
}

func NewWithBurst(requestsPerSecond float64, burst int) *Limiter {
	if requestsPerSecond <= 0 {
		requestsPerSecond = 1
	}
	if burst <= 0 {
		burst = 1
	}

	return &Limiter{limiter: rate.NewLimiter(rate.Limit(requestsPerSecond), burst)}
}

func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

func (l *Limiter) Allow() bool {
	return l.limiter.Allow()
}

func (l *Limiter) SetLimit(requestsPerSecond float64) {
	if requestsPerSecond <= 0 {
		return
	}
	l.limiter.SetLimit(rate.Limit(requestsPerSecond))
}

func (l *Limiter) SetBurst(burst int) {
	if burst <= 0 {
		return
	}
	l.limiter.SetBurst(burst)
}
