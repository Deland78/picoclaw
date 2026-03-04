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
	time.Sleep(350 * time.Millisecond) // ~3 frames
	s.Stop()

	out := buf.String()
	if !strings.Contains(out, "Working...") {
		t.Errorf("expected label in output, got %q", out)
	}
	// Should end with a clear-line sequence
	if !strings.HasSuffix(out, "\033[K") {
		t.Errorf("expected line clear at end, got suffix %q", out[max(0, len(out)-10):])
	}
}

func TestSpinnerDoubleStart(t *testing.T) {
	var buf bytes.Buffer
	s := NewSpinner(&buf)

	s.Start("A")
	s.Start("B") // should be no-op
	time.Sleep(150 * time.Millisecond)
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
