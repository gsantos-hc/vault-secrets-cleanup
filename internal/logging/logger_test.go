package logging

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNew_TextLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := New(Config{Level: "debug", Format: "text", Output: buf})
	require.NoError(t, err)

	logger.Info("hello", "key", "value")
	out := buf.String()
	require.Contains(t, out, "hello")
	require.Contains(t, out, "key=value")
}

func TestNew_JSONLogger(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := New(Config{Level: "info", Format: "json", Output: buf})
	require.NoError(t, err)

	logger.Warn("warn-msg", "a", 1)
	out := buf.String()
	require.Contains(t, out, "\"msg\":\"warn-msg\"")
	require.Contains(t, out, "\"a\":1")
}

func TestWithAndWithGroup(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := New(Config{Level: "debug", Format: "json", Output: buf})
	require.NoError(t, err)

	logger.With("component", "validate").WithGroup("vault").Info("ok", "namespace", "team-a")
	out := buf.String()
	require.Contains(t, out, "\"component\":\"validate\"")
	require.Contains(t, out, "\"vault\":{")
	require.Contains(t, out, "\"namespace\":\"team-a\"")
}

func TestContextMethods(t *testing.T) {
	buf := &bytes.Buffer{}
	logger, err := New(Config{Level: "debug", Format: "text", Output: buf})
	require.NoError(t, err)

	ctx := context.Background()
	logger.DebugContext(ctx, "ctx-msg")
	require.True(t, strings.Contains(buf.String(), "ctx-msg"))
}
