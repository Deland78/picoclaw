package cli

import (
	"fmt"
	"sync"
	"time"
)

type usageEntry struct {
	ts         time.Time
	prompt     int
	completion int
	total      int
}

// UsageTracker accumulates token usage with timestamps.
type UsageTracker struct {
	mu      sync.Mutex
	entries []usageEntry
	nowFunc func() time.Time // for testing
}

// NewUsageTracker creates a new tracker.
func NewUsageTracker() *UsageTracker {
	return &UsageTracker{nowFunc: time.Now}
}

// Record adds a usage entry.
func (t *UsageTracker) Record(prompt, completion, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.entries = append(t.entries, usageEntry{
		ts:         t.nowFunc(),
		prompt:     prompt,
		completion: completion,
		total:      total,
	})
}

// Since returns aggregated (prompt, completion, total) tokens within the given duration.
func (t *UsageTracker) Since(d time.Duration) (prompt, completion, total int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := t.nowFunc().Add(-d)
	for _, e := range t.entries {
		if e.ts.After(cutoff) {
			prompt += e.prompt
			completion += e.completion
			total += e.total
		}
	}
	return
}

// Prune removes entries older than 25 hours.
func (t *UsageTracker) Prune() {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := t.nowFunc().Add(-25 * time.Hour)
	n := 0
	for _, e := range t.entries {
		if e.ts.After(cutoff) {
			t.entries[n] = e
			n++
		}
	}
	t.entries = t.entries[:n]
}

// FormatStatusLine returns a dim status string like:
// claude-sonnet-4.6 | 1h: 12.5k tokens | 24h: 45.2k tokens
func (t *UsageTracker) FormatStatusLine(model string) string {
	_, _, h1 := t.Since(1 * time.Hour)
	_, _, h24 := t.Since(24 * time.Hour)
	return fmt.Sprintf("%s | 1h: %s tokens | 24h: %s tokens", model, fmtTokens(h1), fmtTokens(h24))
}

func fmtTokens(n int) string {
	if n >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	}
	if n >= 1_000 {
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	}
	return fmt.Sprintf("%d", n)
}
