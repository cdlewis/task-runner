package main

import (
	"encoding/json"
	"testing"
)

func TestInterpolatePrompt(t *testing.T) {
	// Helper to create a candidate from JSON
	makeCandidate := func(jsonStr string) *Candidate {
		candidates, _ := ParseCandidates([]byte("[" + jsonStr + "]"))
		return &candidates[0]
	}

	t.Run("$INPUT with single string", func(t *testing.T) {
		c := makeCandidate(`"hello"`)
		result := InterpolatePrompt("Say: $INPUT", c)
		if result != "Say: hello" {
			t.Errorf("got %q, want %q", result, "Say: hello")
		}
	})

	t.Run("$INPUT with single-item array unwraps", func(t *testing.T) {
		c := makeCandidate(`["only_item"]`)
		result := InterpolatePrompt("Value: $INPUT", c)
		if result != "Value: only_item" {
			t.Errorf("got %q, want %q", result, "Value: only_item")
		}
	})

	t.Run("$INPUT with multi-item array returns JSON", func(t *testing.T) {
		c := makeCandidate(`["a", "b", "c"]`)
		result := InterpolatePrompt("Values: $INPUT", c)
		if result != `Values: ["a", "b", "c"]` {
			t.Errorf("got %q, want %q", result, `Values: ["a", "b", "c"]`)
		}
	})

	t.Run("$INPUT[0] array index", func(t *testing.T) {
		c := makeCandidate(`["first", "second", "third"]`)
		result := InterpolatePrompt("First: $INPUT[0]", c)
		if result != "First: first" {
			t.Errorf("got %q, want %q", result, "First: first")
		}
	})

	t.Run("$INPUT[1] array index", func(t *testing.T) {
		c := makeCandidate(`["first", "second", "third"]`)
		result := InterpolatePrompt("Second: $INPUT[1]", c)
		if result != "Second: second" {
			t.Errorf("got %q, want %q", result, "Second: second")
		}
	})

	t.Run("$INPUT[n] out of bounds returns empty", func(t *testing.T) {
		c := makeCandidate(`["only"]`)
		result := InterpolatePrompt("Missing: $INPUT[5]", c)
		if result != "Missing: " {
			t.Errorf("got %q, want %q", result, "Missing: ")
		}
	})

	t.Run("$INPUT[1:] slice from index", func(t *testing.T) {
		c := makeCandidate(`["a", "b", "c", "d"]`)
		result := InterpolatePrompt("Rest: $INPUT[1:]", c)
		if result != `Rest: ["b","c","d"]` {
			t.Errorf("got %q, want %q", result, `Rest: ["b","c","d"]`)
		}
	})

	t.Run("$INPUT[n:] slice out of bounds returns empty array", func(t *testing.T) {
		c := makeCandidate(`["a"]`)
		result := InterpolatePrompt("Rest: $INPUT[5:]", c)
		if result != "Rest: []" {
			t.Errorf("got %q, want %q", result, "Rest: []")
		}
	})

	t.Run("$INPUT[\"key\"] map access", func(t *testing.T) {
		c := makeCandidate(`{"file": "test.go", "line": 42}`)
		result := InterpolatePrompt("File: $INPUT[\"file\"], Line: $INPUT[\"line\"]", c)
		if result != "File: test.go, Line: 42" {
			t.Errorf("got %q, want %q", result, "File: test.go, Line: 42")
		}
	})

	t.Run("$INPUT[\"key\"] missing key returns empty", func(t *testing.T) {
		c := makeCandidate(`{"file": "test.go"}`)
		result := InterpolatePrompt("Missing: $INPUT[\"nope\"]", c)
		if result != "Missing: " {
			t.Errorf("got %q, want %q", result, "Missing: ")
		}
	})

	t.Run("mixed syntax in same template", func(t *testing.T) {
		c := makeCandidate(`["a", "b", "c"]`)
		result := InterpolatePrompt("All: $INPUT, First: $INPUT[0], Rest: $INPUT[1:]", c)
		expected := `All: ["a", "b", "c"], First: a, Rest: ["b","c"]`
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("$INPUT does not match $INPUTX", func(t *testing.T) {
		c := makeCandidate(`"test"`)
		result := InterpolatePrompt("$INPUTX $INPUT", c)
		if result != "$INPUTX test" {
			t.Errorf("got %q, want %q", result, "$INPUTX test")
		}
	})
}

func TestInterpolateCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		key      string
		taskName string
		expected string
	}{
		{
			name:     "replace $CANDIDATE",
			command:  "echo $CANDIDATE",
			key:      "file.go:10",
			taskName: "lint",
			expected: "echo file.go:10",
		},
		{
			name:     "replace $TASK_NAME",
			command:  "run-$TASK_NAME.sh",
			key:      "test",
			taskName: "build",
			expected: "run-build.sh",
		},
		{
			name:     "replace both",
			command:  "$TASK_NAME: $CANDIDATE",
			key:      "error",
			taskName: "fix",
			expected: "fix: error",
		},
		{
			name:     "JSON key for array candidate",
			command:  "git commit -m 'fix $CANDIDATE'",
			key:      `["file.go","line 10"]`,
			taskName: "fix",
			expected: `git commit -m 'fix ["file.go","line 10"]'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := &Candidate{
				Key:  tt.key,
				Data: json.RawMessage(`"placeholder"`),
			}
			result := InterpolateCommand(tt.command, candidate, tt.taskName)
			if result != tt.expected {
				t.Errorf("InterpolateCommand() = %q, want %q", result, tt.expected)
			}
		})
	}
}
