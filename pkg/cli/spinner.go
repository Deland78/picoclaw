package cli

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
)

var spinnerFrames = []string{"|", "/", "-", "\\"}

// Spinner displays an ASCII spinner on a single line.
type Spinner struct {
	w      io.Writer
	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
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
	}
}

func (s *Spinner) run(ctx context.Context, label string) {
	defer close(s.done)
	i := 0
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(s.w, "\r\033[K") // clear line
			return
		case <-ticker.C:
			frame := spinnerFrames[i%len(spinnerFrames)]
			fmt.Fprintf(s.w, "\r%s %s", frame, label)
			i++
		}
	}
}
