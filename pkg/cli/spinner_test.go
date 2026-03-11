package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestSpinnerStartStop(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)

	s.Start("Working...")
	time.Sleep(1500 * time.Millisecond) // wait for at least one tick
	s.Stop()

	out := buf.String()
	if !strings.Contains(out, "Working...") {
		t.Errorf("expected label in output, got %q", out)
	}
	if !strings.Contains(out, "[00:") {
		t.Errorf("expected timer format [00:xx] in output, got %q", out)
	}
	// Should end with a clear-line sequence
	if !strings.HasSuffix(out, "\033[K") {
		t.Errorf("expected line clear at end, got suffix %q", out[max(0, len(out)-10):])
	}
}

func TestSpinnerElapsed(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)

	// Before start, elapsed should be zero
	if d := s.Elapsed(); d != 0 {
		t.Errorf("Elapsed() before Start = %v, want 0", d)
	}

	s.Start("Working...")
	time.Sleep(1500 * time.Millisecond)
	s.Stop()

	d := s.Elapsed()
	if d < 1*time.Second || d > 3*time.Second {
		t.Errorf("Elapsed() after Stop = %v, want ~1.5s", d)
	}
}

func TestSpinnerDoubleStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)

	s.Start("A")
	s.Start("B") // should be no-op
	time.Sleep(1500 * time.Millisecond)
	s.Stop()

	out := buf.String()
	if strings.Contains(out, "B") {
		t.Error("second Start should have been a no-op")
	}
}

func TestSpinnerStopWithoutStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)
	s.Stop() // should not panic
	if buf.Len() != 0 {
		t.Error("stop without start should produce no output")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
