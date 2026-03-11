package cli

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

// Spinner displays an elapsed timer on a single line.
type Spinner struct {
	w       io.Writer
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	started time.Time
	elapsed time.Duration
}

// NewSpinner creates a spinner that writes to w.
func NewSpinner(w io.Writer) *Spinner {
	return &Spinner{w: w}
}

// Start begins the spinner with the given label. Safe to call multiple times;
// subsequent calls are no-ops until Stop is called.
func (s *Spinner) Start(label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return // already running
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.done = make(chan struct{})
	s.started = time.Now()
	s.elapsed = 0
	go s.run(ctx, label)
}

// Stop halts the spinner and clears the line. No-op if not running.
func (s *Spinner) Stop() {
	s.mu.Lock()
	cancel := s.cancel
	done := s.done
	s.cancel = nil
	s.done = nil
	s.mu.Unlock()

	if cancel != nil {
		cancel()
		<-done
		s.mu.Lock()
		s.elapsed = time.Since(s.started)
		s.mu.Unlock()
	}
}

// Elapsed returns the duration of the last Start/Stop cycle.
func (s *Spinner) Elapsed() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.elapsed
}

func (s *Spinner) run(ctx context.Context, label string) {
	defer close(s.done)
	start := time.Now()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	// Show 00:00 immediately
	fmt.Fprintf(s.w, "\r[00:00] %s", label)
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(s.w, "\r\033[K") // clear line
			return
		case <-ticker.C:
			elapsed := time.Since(start)
			mins := int(elapsed.Minutes())
			secs := int(elapsed.Seconds()) % 60
			fmt.Fprintf(s.w, "\r[%02d:%02d] %s", mins, secs, label)
		}
	}
}
