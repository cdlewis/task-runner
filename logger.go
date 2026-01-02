package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const separator = "================================================================================"

// Outcome represents the result of processing a candidate.
type Outcome string

const (
	OutcomeFixed         Outcome = "FIXED"
	OutcomeFixedReverted Outcome = "FIXED_BUT_REVERTED" // Fixed but build failed, had to revert
	OutcomeNotFixed      Outcome = "NOT_FIXED"
	OutcomeBestEffort    Outcome = "BEST_EFFORT" // Not fixed but partial progress committed
	OutcomeBuildFailed   Outcome = "BUILD_FAILED"
)

// ClaudeLogger handles logging of Claude interactions.
type ClaudeLogger struct {
	file      *os.File
	startTime time.Time
}

// NewClaudeLogger creates a new logger for Claude interactions.
func NewClaudeLogger(taskDir string) (*ClaudeLogger, error) {
	path := filepath.Join(taskDir, "claude.log")
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open claude log: %w", err)
	}

	return &ClaudeLogger{file: file}, nil
}

// StartEntry begins a new log entry with timestamp and prompt.
func (l *ClaudeLogger) StartEntry(prompt string) error {
	l.startTime = time.Now()
	timestamp := l.startTime.Format("2006-01-02 15:04:05")

	_, err := fmt.Fprintf(l.file, "\n%s\nTimestamp: %s\nPrompt: %s\n%s\n",
		separator, timestamp, prompt, separator)
	return err
}

// LogOutcome logs the result of processing the candidate.
func (l *ClaudeLogger) LogOutcome(outcome Outcome, details string) error {
	duration := time.Since(l.startTime)
	_, err := fmt.Fprintf(l.file, "\n%s\nOutcome: %s\nDuration: %s\nDetails: %s\n",
		separator, outcome, formatDuration(duration), details)
	return err
}

// EndEntry closes the current log entry.
func (l *ClaudeLogger) EndEntry() error {
	_, err := fmt.Fprintf(l.file, "%s\n", separator)
	return err
}

// Write implements io.Writer for streaming Claude output to the log.
func (l *ClaudeLogger) Write(p []byte) (n int, err error) {
	return l.file.Write(p)
}

// Close closes the log file.
func (l *ClaudeLogger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// Path returns the path to the log file.
func (l *ClaudeLogger) Path() string {
	return l.file.Name()
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", minutes, seconds)
}
