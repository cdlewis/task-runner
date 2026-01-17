package main

import (
	"testing"
	"time"
)

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		expected time.Duration
	}{
		{
			name:     "level 0 returns base backoff",
			level:    0,
			expected: 5 * time.Minute,
		},
		{
			name:     "level 1 doubles",
			level:    1,
			expected: 10 * time.Minute,
		},
		{
			name:     "level 2 quadruples",
			level:    2,
			expected: 20 * time.Minute,
		},
		{
			name:     "level 3 is 40 minutes",
			level:    3,
			expected: 40 * time.Minute,
		},
		{
			name:     "level 4 caps at max",
			level:    4,
			expected: 1 * time.Hour,
		},
		{
			name:     "level 5 stays at max",
			level:    5,
			expected: 1 * time.Hour,
		},
		{
			name:     "level 10 stays at max",
			level:    10,
			expected: 1 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateBackoff(tt.level)
			if result != tt.expected {
				t.Errorf("calculateBackoff(%d) = %v, want %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestRateLimitError(t *testing.T) {
	err := &rateLimitError{msg: "test rate limit"}

	if err.Error() != "test rate limit" {
		t.Errorf("rateLimitError.Error() = %q, want %q", err.Error(), "test rate limit")
	}

	// Verify it implements error interface
	var _ error = err
}

func TestCandidateVerification(t *testing.T) {
	tests := []struct {
		name          string
		candidateKey  string
		sourceOutput  string
		hashFilter    HashFilter
		expectedFixed bool
	}{
		{
			name:          "candidate present in JSON array - not fixed",
			candidateKey:  "func_800BB754_ABF84",
			sourceOutput:  `["func_800BB754_ABF84", "func_800BB66C_B2C2C"]`,
			hashFilter:    HashFilterNone,
			expectedFixed: false,
		},
		{
			name:          "candidate absent from JSON array - fixed",
			candidateKey:  "func_800BB754_ABF84",
			sourceOutput:  `["func_800BB66C_B2C2C", "other_function"]`,
			hashFilter:    HashFilterNone,
			expectedFixed: true,
		},
		{
			name:          "candidate present in newline-separated output - not fixed",
			candidateKey:  "func_800BB754_ABF84",
			sourceOutput:  "func_800BB754_ABF84\nfunc_800BB66C_B2C2C\n",
			hashFilter:    HashFilterNone,
			expectedFixed: false,
		},
		{
			name:          "candidate absent from newline-separated output - fixed",
			candidateKey:  "func_800BB754_ABF84",
			sourceOutput:  "func_800BB66C_B2C2C\nother_function\n",
			hashFilter:    HashFilterNone,
			expectedFixed: true,
		},
		{
			name:          "object candidates - candidate present",
			candidateKey:  `{"file":"foo.c","line":42}`,
			sourceOutput:  `[{"file":"foo.c","line":42},{"file":"bar.c","line":10}]`,
			hashFilter:    HashFilterNone,
			expectedFixed: false,
		},
		{
			name:          "object candidates - candidate absent",
			candidateKey:  `{"file":"foo.c","line":42}`,
			sourceOutput:  `[{"file":"bar.c","line":10},{"file":"baz.c","line":99}]`,
			hashFilter:    HashFilterNone,
			expectedFixed: true,
		},
		{
			name:          "object candidates - key order normalized",
			candidateKey:  `{"file":"foo.c","line":42}`,
			sourceOutput:  `[{"line":42,"file":"foo.c"},{"file":"bar.c","line":10}]`,
			hashFilter:    HashFilterNone,
			expectedFixed: false,
		},
		{
			name:          "array candidate present in array-of-arrays source",
			candidateKey:  `["a","b"]`,
			sourceOutput:  `[["b","z"],["a","b"]]`,
			hashFilter:    HashFilterNone,
			expectedFixed: false,
		},
		{
			name:          "array candidate absent from array-of-arrays source",
			candidateKey:  `["a","b"]`,
			sourceOutput:  `[["b","z"],["c","d"]]`,
			hashFilter:    HashFilterNone,
			expectedFixed: true,
		},
		{
			name:          "array candidate with whitespace - compaction normalizes",
			candidateKey:  `["a","b"]`,
			sourceOutput:  `[["b","z"], [ "a", "b" ]]`,
			hashFilter:    HashFilterNone,
			expectedFixed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates, err := ParseCandidates([]byte(tt.sourceOutput))
			if err != nil {
				t.Fatalf("ParseCandidates failed: %v", err)
			}

			candidates = FilterByHash(candidates, tt.hashFilter)

			fixed := !containsKey(candidates, tt.candidateKey)
			if fixed != tt.expectedFixed {
				t.Errorf("containsKey result = %v (fixed), want %v. Candidate key: %q, Parsed candidates: %+v",
					fixed, tt.expectedFixed, tt.candidateKey, candidates)
			}
		})
	}
}

func TestCandidateVerificationWithHashFilter(t *testing.T) {
	// Test that hash filtering is applied consistently
	// This verifies the fix for the hash filter inconsistency bug
	sourceOutput := `["candidate1", "candidate2", "candidate3", "candidate4", "candidate5"]`

	candidates, err := ParseCandidates([]byte(sourceOutput))
	if err != nil {
		t.Fatalf("ParseCandidates failed: %v", err)
	}

	// Find a candidate that will be filtered out by evens filter
	var filteredOutKey string
	for _, c := range candidates {
		evensFiltered := FilterByHash([]Candidate{c}, HashFilterEvens)
		if len(evensFiltered) == 0 {
			filteredOutKey = c.Key
			break
		}
	}

	if filteredOutKey == "" {
		t.Skip("Could not find a candidate that gets filtered by evens (need more candidates)")
	}

	// With evens filter, the filtered-out candidate should appear "fixed" (not found)
	evensCandidates := FilterByHash(candidates, HashFilterEvens)
	fixedWithEvens := !containsKey(evensCandidates, filteredOutKey)
	if !fixedWithEvens {
		t.Errorf("Candidate %q should be filtered out by evens filter", filteredOutKey)
	}

	// With odds filter, the same candidate should be present (not "fixed")
	oddsCandidates := FilterByHash(candidates, HashFilterOdds)
	fixedWithOdds := !containsKey(oddsCandidates, filteredOutKey)
	if fixedWithOdds {
		t.Errorf("Candidate %q should be present in odds filter", filteredOutKey)
	}

	// With no filter, candidate should be present
	noFilterCandidates := FilterByHash(candidates, HashFilterNone)
	fixedWithNoFilter := !containsKey(noFilterCandidates, filteredOutKey)
	if fixedWithNoFilter {
		t.Errorf("Candidate %q should be present with no filter", filteredOutKey)
	}
}

func TestGetPromptMissingTemplateIsFatal(t *testing.T) {
	// Create a minimal environment for testing
	env := &Environment{
		ProjectDir: "/tmp/test-project",
		Config: Config{
			ClaudeCommand: "claude",
		},
		Tasks: map[string]Task{
			"test-task": {
				Name:     "test-task",
				Dir:      "/tmp/test-task",
				Template: "nonexistent.md",
			},
		},
	}

	// Use DryRun to avoid creating a logger (which requires directories to exist)
	runner, err := NewRunner(env, "test-task", RunnerOptions{DryRun: true})
	if err != nil {
		t.Fatalf("NewRunner failed: %v", err)
	}

	candidate := &Candidate{Key: "test-candidate"}
	_, err = runner.getPrompt(candidate)

	// Error should be a fatalError
	if _, isFatal := err.(*fatalError); !isFatal {
		t.Errorf("getPrompt with missing template should return fatalError, got %T", err)
	}
}
