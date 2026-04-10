package deletion

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProgressTracker_TracksCountsAndPercentage(t *testing.T) {
	p := NewProgressTracker(4)
	p.RecordSuccess()
	p.RecordFailure()

	completed, failed, total, pct := p.GetProgress()
	require.Equal(t, 2, completed)
	require.Equal(t, 1, failed)
	require.Equal(t, 4, total)
	require.Equal(t, 50.0, pct)
}

func TestProgressTracker_Eta(t *testing.T) {
	p := NewProgressTracker(2)
	time.Sleep(5 * time.Millisecond)
	p.RecordSuccess()

	require.Greater(t, p.GetETA(), time.Duration(0))
}
