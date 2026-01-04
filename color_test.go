package main

import (
	"strings"
	"testing"
)

func TestColorFunctions(t *testing.T) {
	tests := []struct {
		name     string
		fn       func(string) string
		input    string
		contains string
	}{
		{"ColorSuccess adds green", ColorSuccess, "test", "\033[32m"},
		{"ColorError adds red", ColorError, "test", "\033[31m"},
		{"ColorWarning adds yellow", ColorWarning, "test", "\033[33m"},
		{"ColorInfo adds cyan", ColorInfo, "test", "\033[36m"},
		{"ColorBold adds bold", ColorBold, "test", "\033[1m"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.fn(tt.input)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("%s: expected result to contain %q, got %q", tt.name, tt.contains, result)
			}
			if !strings.Contains(result, tt.input) {
				t.Errorf("%s: expected result to contain input %q, got %q", tt.name, tt.input, result)
			}
			if !strings.HasSuffix(result, colorReset) {
				t.Errorf("%s: expected result to end with reset code", tt.name)
			}
		})
	}
}

func TestGradient(t *testing.T) {
	result := Gradient("Hello")

	// Should contain at least one color code
	if !strings.Contains(result, "\033[") {
		t.Error("Gradient should contain ANSI color codes")
	}

	// Should end with reset
	if !strings.HasSuffix(result, colorReset) {
		t.Error("Gradient should end with reset code")
	}

	// Should contain the original text characters
	if !strings.Contains(result, "H") || !strings.Contains(result, "o") {
		t.Error("Gradient should preserve original characters")
	}
}

func TestIterationBanner(t *testing.T) {
	result := IterationBanner(1, "14:30:05")

	// Should contain iteration text with sparkles
	if !strings.Contains(result, "Iteration 1") {
		t.Error("Banner should contain 'Iteration 1' text")
	}

	// Should contain sparkles
	if !strings.Contains(result, "✦") {
		t.Error("Banner should contain sparkle decorations")
	}

	// Should contain time
	if !strings.Contains(result, "14:30:05") {
		t.Error("Banner should contain time")
	}

	// Should contain box-drawing characters
	if !strings.Contains(result, "╔") || !strings.Contains(result, "╚") {
		t.Error("Banner should contain box-drawing characters")
	}

	// Should start with newline
	if !strings.HasPrefix(result, "\n") {
		t.Error("Banner should start with newline")
	}

	// Should contain cyan color code for border
	if !strings.Contains(result, colorCyan) {
		t.Error("Banner should contain cyan color for border")
	}

	// Should contain bold code for text
	if !strings.Contains(result, colorBold) {
		t.Error("Banner should contain bold formatting for text")
	}
}

func TestStartupBanner(t *testing.T) {
	result := StartupBanner("my-task", "/path/to/logs", "standard")

	// Should contain task name with label
	if !strings.Contains(result, "Task: my-task") {
		t.Error("Startup banner should contain 'Task: my-task'")
	}

	// Should contain mode with label
	if !strings.Contains(result, "Mode: standard") {
		t.Error("Startup banner should contain 'Mode: standard'")
	}

	// Should contain log path with label
	if !strings.Contains(result, "Logs: /path/to/logs") {
		t.Error("Startup banner should contain 'Logs: /path/to/logs'")
	}

	// Should contain bold "Nigel"
	if !strings.Contains(result, colorBold+"Nigel") {
		t.Error("Startup banner should contain bold 'Nigel'")
	}

	// Should contain cat ASCII art elements
	if !strings.Contains(result, "フ") || !strings.Contains(result, "ミ") {
		t.Error("Startup banner should contain cat ASCII art")
	}

	// Should contain cyan color
	if !strings.Contains(result, colorCyan) {
		t.Error("Startup banner should use cyan color for cat")
	}
}

func TestStartupBannerDryRun(t *testing.T) {
	result := StartupBanner("my-task", "/path/to/claude.log", "dry-run")

	// Should contain task name
	if !strings.Contains(result, "my-task") {
		t.Error("Startup banner should contain task name")
	}

	// Should contain logs path even in dry-run
	if !strings.Contains(result, "Logs:") || !strings.Contains(result, "claude.log") {
		t.Error("Startup banner should show log path in dry-run mode")
	}

	// Should show dry-run mode
	if !strings.Contains(result, "dry-run") {
		t.Error("Startup banner should show dry-run mode")
	}
}
