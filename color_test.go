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

	// Should contain iteration number (note: gradient adds color codes between chars)
	if !strings.Contains(result, "I") || !strings.Contains(result, "t") {
		t.Error("Banner should contain 'Iteration' text")
	}

	// Should contain time digits (gradient splits them with color codes)
	if !strings.Contains(result, "1") || !strings.Contains(result, "4") || !strings.Contains(result, "3") {
		t.Error("Banner should contain time digits")
	}

	// Should contain box-drawing characters
	if !strings.Contains(result, "╔") || !strings.Contains(result, "╚") {
		t.Error("Banner should contain box-drawing characters")
	}

	// Should start with newline
	if !strings.HasPrefix(result, "\n") {
		t.Error("Banner should start with newline")
	}

	// Should contain color codes (from gradient)
	if !strings.Contains(result, "\033[") {
		t.Error("Banner should contain ANSI color codes")
	}
}
