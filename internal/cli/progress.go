// Copyright IBM Corp. 2026
// SPDX-License-Identifier: MIT

package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type progressMode string

const (
	progressModeAuto progressMode = "auto"
	progressModeTTY  progressMode = "tty"
	progressModeLog  progressMode = "log"
	progressModeOff  progressMode = "off"
)

type progressSnapshot struct {
	Operation string
	Phase     string
	Completed int
	Total     int
	Failed    int
	Elapsed   time.Duration
	ETA       time.Duration
}

type progressRenderer interface {
	Render(snapshot progressSnapshot)
	Close()
}

type logProgressRenderer struct {
	out io.Writer
}

func newLogProgressRenderer(out io.Writer) *logProgressRenderer {
	return &logProgressRenderer{out: out}
}

func (r *logProgressRenderer) Render(snapshot progressSnapshot) {
	if r == nil || r.out == nil {
		return
	}
	_, _ = fmt.Fprintf(
		r.out,
		"progress operation=%s phase=%s completed=%d total=%d failed=%d elapsed=%s eta=%s\n",
		snapshot.Operation,
		snapshot.Phase,
		snapshot.Completed,
		snapshot.Total,
		snapshot.Failed,
		snapshot.Elapsed.Round(time.Second),
		snapshot.ETA.Round(time.Second),
	)
}

func (r *logProgressRenderer) Close() {}

type ttyProgressRenderer struct {
	out io.Writer
}

func newTTYProgressRenderer(out io.Writer) *ttyProgressRenderer {
	return &ttyProgressRenderer{out: out}
}

func (r *ttyProgressRenderer) Render(snapshot progressSnapshot) {
	if r == nil || r.out == nil {
		return
	}
	line := fmt.Sprintf(
		"%s %s %d/%d failed=%d elapsed=%s eta=%s",
		snapshot.Operation,
		snapshot.Phase,
		snapshot.Completed,
		snapshot.Total,
		snapshot.Failed,
		snapshot.Elapsed.Round(time.Second),
		snapshot.ETA.Round(time.Second),
	)
	_, _ = fmt.Fprintf(r.out, "\r%s", line)
}

func (r *ttyProgressRenderer) Close() {
	if r == nil || r.out == nil {
		return
	}
	_, _ = fmt.Fprintln(r.out)
}

type progressReporter struct {
	mu        sync.Mutex
	operation string
	mode      progressMode
	renderer  progressRenderer
	start     time.Time
	interval  time.Duration
	nextEmit  time.Time
}

func newProgressReporter(out io.Writer, operation string) (*progressReporter, error) {
	requested, err := parseProgressMode(GetProgressMode())
	if err != nil {
		return nil, err
	}
	resolved, warning := resolveProgressMode(requested, isTTYWriter(out))
	if warning != "" {
		_, _ = fmt.Fprintln(out, warning)
	}

	r := &progressReporter{operation: operation, mode: resolved, start: time.Now()}
	switch resolved {
	case progressModeTTY:
		r.renderer = newTTYProgressRenderer(out)
		r.interval = 250 * time.Millisecond
	case progressModeLog:
		r.renderer = newLogProgressRenderer(out)
		r.interval = 2 * time.Second
	default:
		r.interval = 0
	}
	r.nextEmit = time.Now()
	return r, nil
}

func (r *progressReporter) Emit(phase string, completed, total, failed int, eta time.Duration) {
	if r == nil || r.mode == progressModeOff || r.renderer == nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Before(r.nextEmit) {
		return
	}
	r.nextEmit = now.Add(r.interval)

	r.renderer.Render(progressSnapshot{
		Operation: r.operation,
		Phase:     phase,
		Completed: completed,
		Total:     total,
		Failed:    failed,
		Elapsed:   time.Since(r.start),
		ETA:       eta,
	})
}

func (r *progressReporter) Close() {
	if r == nil || r.renderer == nil {
		return
	}
	r.renderer.Close()
}

func isTTYWriter(out io.Writer) bool {
	file, ok := out.(*os.File)
	if !ok {
		return false
	}
	stat, err := file.Stat()
	if err != nil {
		return false
	}
	return stat.Mode()&os.ModeCharDevice != 0
}

func parseProgressMode(raw string) (progressMode, error) {
	mode := progressMode(strings.TrimSpace(strings.ToLower(raw)))
	switch mode {
	case progressModeAuto, progressModeTTY, progressModeLog, progressModeOff:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid progress mode %q: must be one of auto, tty, log, off", raw)
	}
}

func resolveProgressMode(requested progressMode, ttyAvailable bool) (progressMode, string) {
	switch requested {
	case progressModeAuto:
		if ttyAvailable {
			return progressModeTTY, ""
		}
		return progressModeLog, ""
	case progressModeTTY:
		if ttyAvailable {
			return progressModeTTY, ""
		}
		return progressModeLog, "TTY progress requested but stdout is not a terminal; falling back to log progress"
	case progressModeLog:
		return progressModeLog, ""
	case progressModeOff:
		return progressModeOff, ""
	default:
		return progressModeOff, ""
	}
}
