package main

import (
	"fmt"
	"os"
	"sort"
	"sync"
	"time"
)

// SessionStats tracks durations across a session for computing statistics.
type SessionStats struct {
	mu        sync.Mutex
	durations []time.Duration
}

// NewSessionStats creates a new SessionStats tracker.
func NewSessionStats() *SessionStats {
	return &SessionStats{
		durations: make([]time.Duration, 0),
	}
}

// Add records a duration.
func (s *SessionStats) Add(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.durations = append(s.durations, d)
}

// Median returns the median duration, or false if no durations recorded.
func (s *SessionStats) Median() (time.Duration, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.durations) == 0 {
		return 0, false
	}

	sorted := make([]time.Duration, len(s.durations))
	copy(sorted, s.durations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2, true
	}
	return sorted[mid], true
}

// ProgressTimer displays a live updating timer during long operations.
type ProgressTimer struct {
	label     string
	startTime time.Time
	stats     *SessionStats
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewProgressTimer creates a new timer with the given label.
func NewProgressTimer(label string, stats *SessionStats) *ProgressTimer {
	return &ProgressTimer{
		label:  label,
		stats:  stats,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start begins the timer display. Call Stop() when the operation completes.
func (p *ProgressTimer) Start() {
	p.startTime = time.Now()

	// Hide cursor
	fmt.Fprint(os.Stdout, "\033[?25l")

	go func() {
		defer close(p.doneCh)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		// Print initial state immediately
		p.printProgress()

		for {
			select {
			case <-p.stopCh:
				return
			case <-ticker.C:
				p.printProgress()
			}
		}
	}()
}

func (p *ProgressTimer) printProgress() {
	elapsed := time.Since(p.startTime)

	var timerPart string
	if median, ok := p.stats.Median(); ok {
		timerPart = fmt.Sprintf("(%s · median time %s)",
			formatDuration(elapsed),
			formatDuration(median))
	} else {
		timerPart = fmt.Sprintf("(%s)", formatDuration(elapsed))
	}

	// \r moves to start of line, \033[K clears to end of line
	fmt.Fprintf(os.Stdout, "\r%s %s\033[K", ColorInfo(p.label), ColorDim(timerPart))
}

// Stop stops the timer and records the duration. Returns the elapsed duration.
func (p *ProgressTimer) Stop() time.Duration {
	close(p.stopCh)
	<-p.doneCh // Wait for goroutine to exit

	duration := time.Since(p.startTime)
	p.stats.Add(duration)

	// Clear the line and show cursor
	fmt.Fprintf(os.Stdout, "\r\033[K\033[?25h")

	return duration
}
