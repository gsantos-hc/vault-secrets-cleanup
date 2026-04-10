package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewAndAllow(t *testing.T) {
	limiter := New(10)
	require.NotNil(t, limiter)
	require.True(t, limiter.Allow())
}

func TestWait_ContextCancelled(t *testing.T) {
	limiter := NewWithBurst(1, 1)
	require.True(t, limiter.Allow())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := limiter.Wait(ctx)
	require.Error(t, err)
}

func TestSetLimitAndBurst(t *testing.T) {
	limiter := NewWithBurst(1, 1)
	limiter.SetLimit(100)
	limiter.SetBurst(10)
	require.True(t, limiter.Allow())
}

func TestWait_AllowsAfterInterval(t *testing.T) {
	limiter := NewWithBurst(10, 1)
	require.True(t, limiter.Allow())
	require.False(t, limiter.Allow())

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := limiter.Wait(ctx)
	require.NoError(t, err)
}
