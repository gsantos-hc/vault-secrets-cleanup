// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveProgressMode_AutoUsesTTYWhenAvailable(t *testing.T) {
	mode, warn := resolveProgressMode(progressModeAuto, true)
	require.Equal(t, progressModeTTY, mode)
	require.Empty(t, warn)
}

func TestResolveProgressMode_AutoUsesLogsWhenTTYUnavailable(t *testing.T) {
	mode, warn := resolveProgressMode(progressModeAuto, false)
	require.Equal(t, progressModeLog, mode)
	require.Empty(t, warn)
}

func TestResolveProgressMode_ForcedTTYFallsBackToLogs(t *testing.T) {
	mode, warn := resolveProgressMode(progressModeTTY, false)
	require.Equal(t, progressModeLog, mode)
	require.Contains(t, warn, "TTY progress requested")
}

func TestParseProgressMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		mode    progressMode
		wantErr bool
	}{
		{name: "auto", input: "auto", mode: progressModeAuto},
		{name: "tty", input: "tty", mode: progressModeTTY},
		{name: "log", input: "log", mode: progressModeLog},
		{name: "off", input: "off", mode: progressModeOff},
		{name: "invalid", input: "json", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode, err := parseProgressMode(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.mode, mode)
		})
	}
}

func TestLogRendererWritesPeriodicLine(t *testing.T) {
	buf := &bytes.Buffer{}
	r := newLogProgressRenderer(buf)
	r.Render(progressSnapshot{Operation: "seed", Phase: "write", Completed: 10, Total: 20})
	out := buf.String()
	require.Contains(t, out, "operation=seed")
	require.Contains(t, out, "phase=write")
	require.Contains(t, out, "completed=10")
	require.Contains(t, out, "total=20")
}
