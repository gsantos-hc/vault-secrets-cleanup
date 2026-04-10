package retry

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type Config struct {
	MaxAttempts  int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
}

func DefaultConfig() Config {
	return Config{
		MaxAttempts:  3,
		InitialDelay: time.Second,
		MaxDelay:     4 * time.Second,
		Multiplier:   2,
	}
}

func Do(ctx context.Context, cfg Config, operation func() error) error {
	return DoWithRetryable(ctx, cfg, operation, func(error) bool { return true })
}

func DoWithRetryable(ctx context.Context, cfg Config, operation func() error, retryable func(error) bool) error {
	cfg = normalize(cfg)
	delay := cfg.InitialDelay

	var lastErr error
	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		err := operation()
		if err == nil {
			return nil
		}
		lastErr = err

		if attempt == cfg.MaxAttempts || !retryable(err) {
			break
		}

		wait := addJitter(delay)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}

		next := time.Duration(float64(delay) * cfg.Multiplier)
		if next > cfg.MaxDelay {
			next = cfg.MaxDelay
		}
		delay = next
	}

	return fmt.Errorf("retry failed after %d attempts: %w", cfg.MaxAttempts, lastErr)
}

func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	type temporary interface {
		Temporary() bool
	}
	var te temporary
	if errors.As(err, &te) && te.Temporary() {
		return true
	}

	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "timeout") || strings.Contains(lower, "connection reset") {
		return true
	}

	for _, code := range []string{"500", "502", "503", "504"} {
		if strings.Contains(lower, code) {
			return true
		}
	}

	return false
}

func normalize(cfg Config) Config {
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 1
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = time.Millisecond
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = cfg.InitialDelay
	}
	if cfg.Multiplier < 1 {
		cfg.Multiplier = 1
	}
	return cfg
}

func addJitter(base time.Duration) time.Duration {
	if base <= 0 {
		return 0
	}
	jitterRange := base / 2
	if jitterRange <= 0 {
		return base
	}
	jitter := time.Duration(rand.Int63n(int64(jitterRange)))
	return base + jitter
}
