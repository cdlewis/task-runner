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
	if p.stats != nil {
		if median, ok := p.stats.Median(); ok {
			timerPart = fmt.Sprintf("(%s · median time %s)",
				formatDuration(elapsed),
				formatDuration(median))
		} else {
			timerPart = fmt.Sprintf("(%s)", formatDuration(elapsed))
		}
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
	if p.stats != nil {
		p.stats.Add(duration)
	}

	// Show cursor and move to new line (keep timer text visible)
	fmt.Fprintf(os.Stdout, "\033[?25h\n")

	return duration
}

// DelayedProgressTimer displays a timer only after the operation exceeds a delay.
type DelayedProgressTimer struct {
	label     string
	delay     time.Duration
	startTime time.Time
	mu        sync.Mutex
	timer     *ProgressTimer
	stopped   bool
}

// NewDelayedProgressTimer creates a new delayed timer with the given label and delay.
func NewDelayedProgressTimer(label string, delay time.Duration) *DelayedProgressTimer {
	return &DelayedProgressTimer{
		label: label,
		delay: delay,
	}
}

// Start begins the delay timer. The actual timer display will only start
// after the delay period has passed (if Stop hasn't been called yet).
func (d *DelayedProgressTimer) Start() {
	d.startTime = time.Now()
	go func() {
		<-time.After(d.delay)
		d.mu.Lock()
		if !d.stopped {
			d.timer = NewProgressTimer(d.label, nil)
			d.timer.Start()
		}
		d.mu.Unlock()
	}()
}

// Stop stops the timer. If the delay hasn't passed yet, no timer is shown.
func (d *DelayedProgressTimer) Stop() {
	d.mu.Lock()
	d.stopped = true
	if d.timer != nil {
		d.timer.Stop()
	}
	d.mu.Unlock()
}
