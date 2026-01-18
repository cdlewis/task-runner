package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
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
	label        string
	startTime    time.Time
	stats        *SessionStats
	stopCh       chan struct{}
	doneCh       chan struct{}
	streamCh     chan string
	lastLine     string // Tracks last streamed line for timer redrawing
	mu           sync.Mutex
}

// NewProgressTimer creates a new timer with the given label.
func NewProgressTimer(label string, stats *SessionStats) *ProgressTimer {
	return &ProgressTimer{
		label:    label,
		stats:    stats,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		streamCh: make(chan string, 100), // Buffer to avoid blocking
	}
}

// Start begins the timer display. Call Stop() when the operation completes.
// Use StreamText() to send text to be displayed above the timer.
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
			case text := <-p.streamCh:
				p.handleStreamText(text)
				// After handling text, ensure timer is redrawn
				p.printProgress()
			}
		}
	}()
}

// StreamText sends text to be displayed above the timer line.
// Safe to call from multiple goroutines.
func (p *ProgressTimer) StreamText(text string) {
	select {
	case p.streamCh <- text:
	case <-p.stopCh:
		// Timer stopped, discard text
	}
}

func (p *ProgressTimer) handleStreamText(text string) {
	if text == "" {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear the timer line
	fmt.Fprint(os.Stdout, "\r\033[K")

	// Print the text immediately
	fmt.Fprint(os.Stdout, text)

	// Ensure we end with newline so timer appears on next line
	if !strings.HasSuffix(text, "\n") {
		fmt.Fprint(os.Stdout, "\n")
	}
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

// Start begins the delay timer. The label will only be shown after the
// delay period has passed, to avoid displaying it for operations that complete quickly.
func (d *DelayedProgressTimer) Start() {
	d.startTime = time.Now()
	go func() {
		<-time.After(d.delay)
		d.mu.Lock()
		if !d.stopped {
			// Print label and start timer only after delay has passed
			fmt.Fprintf(os.Stdout, "%s\033[K", ColorInfo(d.label))
			d.timer = NewProgressTimer(d.label, nil)
			d.timer.Start()
		}
		d.mu.Unlock()
	}()
}

// Stop stops the timer. If the delay hasn't passed yet, just ends the label line.
func (d *DelayedProgressTimer) Stop() {
	d.mu.Lock()
	d.stopped = true
	if d.timer != nil {
		d.timer.Stop()
		d.timer = nil
	} else {
		// No timer was shown, just end the line
		fmt.Fprintf(os.Stdout, "\n")
	}
	d.mu.Unlock()
}

// Reset restarts the inactivity timer. If a timer was being displayed, it will be hidden
// and the delay period starts over.
func (d *DelayedProgressTimer) Reset() {
	d.mu.Lock()
	// Stop any running timer display
	if d.timer != nil {
		// Clear the timer line before stopping (otherwise the timer text stays visible)
		fmt.Fprint(os.Stdout, "\r\033[K")
		d.timer.Stop()
		d.timer = nil
	}
	// Reset stopped state and restart delay
	d.stopped = false
	d.startTime = time.Now()
	d.mu.Unlock()
	// Restart the delay goroutine
	go func() {
		<-time.After(d.delay)
		d.mu.Lock()
		if !d.stopped && d.timer == nil {
			d.timer = NewProgressTimer(d.label, nil)
			d.timer.Start()
		}
		d.mu.Unlock()
	}()
}
