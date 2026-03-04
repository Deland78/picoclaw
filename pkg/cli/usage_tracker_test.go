package cli

import (
	"strings"
	"testing"
	"time"
)

func TestRecordAndSince(t *testing.T) {
	tr := NewUsageTracker()
	now := time.Now()
	tr.nowFunc = func() time.Time { return now }

	tr.Record(100, 50, 150)
	tr.Record(200, 100, 300)

	p, c, total := tr.Since(1 * time.Hour)
	if p != 300 || c != 150 || total != 450 {
		t.Errorf("Since(1h) = (%d, %d, %d), want (300, 150, 450)", p, c, total)
	}
}

func TestSinceFiltersOldEntries(t *testing.T) {
	tr := NewUsageTracker()
	now := time.Now()

	// Record an old entry
	tr.nowFunc = func() time.Time { return now.Add(-2 * time.Hour) }
	tr.Record(1000, 500, 1500)

	// Record a recent entry
	tr.nowFunc = func() time.Time { return now }
	tr.Record(100, 50, 150)

	_, _, total := tr.Since(1 * time.Hour)
	if total != 150 {
		t.Errorf("Since(1h) total = %d, want 150 (old entry should be excluded)", total)
	}

	_, _, total24 := tr.Since(24 * time.Hour)
	if total24 != 1650 {
		t.Errorf("Since(24h) total = %d, want 1650", total24)
	}
}

func TestPrune(t *testing.T) {
	tr := NewUsageTracker()
	now := time.Now()

	// Old entry (26h ago)
	tr.nowFunc = func() time.Time { return now.Add(-26 * time.Hour) }
	tr.Record(999, 999, 1998)

	// Recent entry
	tr.nowFunc = func() time.Time { return now }
	tr.Record(100, 50, 150)

	tr.Prune()

	tr.mu.Lock()
	count := len(tr.entries)
	tr.mu.Unlock()

	if count != 1 {
		t.Errorf("after Prune, entries = %d, want 1", count)
	}

	_, _, total := tr.Since(48 * time.Hour)
	if total != 150 {
		t.Errorf("after Prune, total = %d, want 150", total)
	}
}

func TestFormatStatusLine(t *testing.T) {
	tr := NewUsageTracker()
	now := time.Now()
	tr.nowFunc = func() time.Time { return now }

	tr.Record(5000, 7500, 12500)

	line := tr.FormatStatusLine("claude-sonnet-4.6")
	if !strings.Contains(line, "claude-sonnet-4.6") {
		t.Errorf("status line missing model name: %q", line)
	}
	if !strings.Contains(line, "12.5k") {
		t.Errorf("expected 12.5k in status line: %q", line)
	}
	if !strings.Contains(line, "1h:") && !strings.Contains(line, "24h:") {
		t.Errorf("expected time windows in status line: %q", line)
	}
}

func TestFmtTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{500, "500"},
		{1000, "1.0k"},
		{12500, "12.5k"},
		{1000000, "1.0M"},
		{2500000, "2.5M"},
	}
	for _, tt := range tests {
		got := fmtTokens(tt.n)
		if got != tt.want {
			t.Errorf("fmtTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
