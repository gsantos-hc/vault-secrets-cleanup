// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type tempErr struct{}

func (tempErr) Error() string   { return "temporary" }
func (tempErr) Timeout() bool   { return false }
func (tempErr) Temporary() bool { return true }

func TestDo_SucceedsAfterRetries(t *testing.T) {
	attempts := 0
	cfg := Config{MaxAttempts: 3, InitialDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, Multiplier: 2}

	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("boom")
		}
		return nil
	})

	require.NoError(t, err)
	require.Equal(t, 3, attempts)
}

func TestDo_StopsAtMaxAttempts(t *testing.T) {
	cfg := Config{MaxAttempts: 3, InitialDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, Multiplier: 2}

	err := Do(context.Background(), cfg, func() error {
		return errors.New("always fails")
	})

	require.Error(t, err)
}

func TestDo_RespectsContextCancel(t *testing.T) {
	cfg := Config{MaxAttempts: 10, InitialDelay: 200 * time.Millisecond, MaxDelay: 500 * time.Millisecond, Multiplier: 2}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Do(ctx, cfg, func() error {
		return errors.New("always fails")
	})
	require.Error(t, err)
}

func TestIsRetryable(t *testing.T) {
	require.True(t, IsRetryable(tempErr{}))
	require.True(t, IsRetryable(fmt.Errorf("upstream returned 503")))
	require.False(t, IsRetryable(errors.New("permission denied")))
}

func TestDoWithRetryable(t *testing.T) {
	attempts := 0
	cfg := Config{MaxAttempts: 5, InitialDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, Multiplier: 2}

	err := DoWithRetryable(context.Background(), cfg, func() error {
		attempts++
		if attempts == 1 {
			return errors.New("503 upstream")
		}
		if attempts == 2 {
			return errors.New("403 forbidden")
		}
		return nil
	}, IsRetryable)

	require.Error(t, err)
	require.Equal(t, 2, attempts)
}
