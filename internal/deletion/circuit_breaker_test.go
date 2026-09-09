// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package deletion

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCircuitBreaker_AllowsWhileClosed(t *testing.T) {
	cb := NewCircuitBreaker(3)

	for i := 0; i < 5; i++ {
		require.True(t, cb.Allow())
		cb.RecordSuccess()
	}
}

func TestCircuitBreaker_OpensAfterConsecutiveFailures(t *testing.T) {
	cb := NewCircuitBreaker(3)

	for i := 0; i < 3; i++ {
		require.True(t, cb.Allow())
		cb.RecordFailure(errors.New("boom"))
	}

	require.False(t, cb.Allow())
	require.True(t, cb.IsOpen())
	require.Error(t, cb.Error())
}

func TestCircuitBreaker_ResetsOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(3)

	cb.RecordFailure(errors.New("error 1"))
	cb.RecordFailure(errors.New("error 2"))
	cb.RecordSuccess()

	failures, open, lastErr := cb.GetStats()
	require.Equal(t, 0, failures)
	require.False(t, open)
	require.NoError(t, lastErr)
	require.True(t, cb.Allow())
}
