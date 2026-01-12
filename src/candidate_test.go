package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseCandidates(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectedKey []string // Expected Key for each candidate
		expectError bool
	}{
		{
			name:        "simple string array",
			input:       `["file1.go", "file2.go", "file3.go"]`,
			expectedKey: []string{"file1.go", "file2.go", "file3.go"},
		},
		{
			name:        "array of arrays",
			input:       `[["file1.go", "line 10"], ["file2.go", "line 20"]]`,
			expectedKey: []string{`["file1.go","line 10"]`, `["file2.go","line 20"]`},
		},
		{
			name:        "array of maps",
			input:       `[{"file": "test.go", "line": 10}, {"file": "other.go"}]`,
			expectedKey: []string{`{"file":"test.go","line":10}`, `{"file":"other.go"}`},
		},
		{
			name:        "mixed strings and arrays",
			input:       `["simple.go", ["complex.go", "extra"]]`,
			expectedKey: []string{"simple.go", `["complex.go","extra"]`},
		},
		{
			name:        "empty array",
			input:       `[]`,
			expectedKey: []string{},
		},
		{
			name:        "invalid JSON",
			input:       `not json`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCandidates([]byte(tt.input))
			if tt.expectError {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result) != len(tt.expectedKey) {
				t.Fatalf("got %d candidates, want %d", len(result), len(tt.expectedKey))
			}
			for i, c := range result {
				if c.Key != tt.expectedKey[i] {
					t.Errorf("candidate[%d].Key = %q, want %q", i, c.Key, tt.expectedKey[i])
				}
			}
		})
	}
}

func TestCandidateAccessors(t *testing.T) {
	t.Run("string candidate", func(t *testing.T) {
		candidates, _ := ParseCandidates([]byte(`["hello"]`))
		c := &candidates[0]

		if !c.IsString() {
			t.Error("expected IsString() to be true")
		}
		if c.IsArray() {
			t.Error("expected IsArray() to be false")
		}
		if c.IsMap() {
			t.Error("expected IsMap() to be false")
		}
		if c.String() != "hello" {
			t.Errorf("String() = %q, want %q", c.String(), "hello")
		}
	})

	t.Run("array candidate - GetIndex", func(t *testing.T) {
		candidates, _ := ParseCandidates([]byte(`[["a", "b", "c"]]`))
		c := &candidates[0]

		if !c.IsArray() {
			t.Error("expected IsArray() to be true")
		}

		val, ok := c.GetIndex(0)
		if !ok || val != "a" {
			t.Errorf("GetIndex(0) = %q, %v; want 'a', true", val, ok)
		}

		val, ok = c.GetIndex(1)
		if !ok || val != "b" {
			t.Errorf("GetIndex(1) = %q, %v; want 'b', true", val, ok)
		}

		val, ok = c.GetIndex(10)
		if ok {
			t.Error("GetIndex(10) should return false for out of bounds")
		}
	})

	t.Run("array candidate - GetSlice", func(t *testing.T) {
		candidates, _ := ParseCandidates([]byte(`[["a", "b", "c", "d"]]`))
		c := &candidates[0]

		val, ok := c.GetSlice(1)
		if !ok || val != `["b","c","d"]` {
			t.Errorf("GetSlice(1) = %q, %v; want '[\"b\",\"c\",\"d\"]', true", val, ok)
		}

		val, ok = c.GetSlice(3)
		if !ok || val != `["d"]` {
			t.Errorf("GetSlice(3) = %q, %v; want '[\"d\"]', true", val, ok)
		}

		val, ok = c.GetSlice(10)
		if !ok || val != "[]" {
			t.Errorf("GetSlice(10) = %q, %v; want '[]', true", val, ok)
		}
	})

	t.Run("map candidate - GetKey", func(t *testing.T) {
		candidates, _ := ParseCandidates([]byte(`[{"file": "test.go", "line": 42}]`))
		c := &candidates[0]

		if !c.IsMap() {
			t.Error("expected IsMap() to be true")
		}

		val, ok := c.GetKey("file")
		if !ok || val != "test.go" {
			t.Errorf("GetKey('file') = %q, %v; want 'test.go', true", val, ok)
		}

		val, ok = c.GetKey("line")
		if !ok || val != "42" {
			t.Errorf("GetKey('line') = %q, %v; want '42', true", val, ok)
		}

		val, ok = c.GetKey("missing")
		if ok {
			t.Error("GetKey('missing') should return false")
		}
	})

	t.Run("String() unwraps single-item arrays", func(t *testing.T) {
		candidates, _ := ParseCandidates([]byte(`[["only_item"]]`))
		c := &candidates[0]

		if c.String() != "only_item" {
			t.Errorf("String() = %q, want %q", c.String(), "only_item")
		}
	})

	t.Run("String() returns JSON for multi-item arrays", func(t *testing.T) {
		candidates, _ := ParseCandidates([]byte(`[["a", "b"]]`))
		c := &candidates[0]

		if c.String() != `["a", "b"]` {
			t.Errorf("String() = %q, want %q", c.String(), `["a", "b"]`)
		}
	})
}

func TestFilterByHash(t *testing.T) {
	candidates := []Candidate{
		{Key: "a"},
		{Key: "b"},
		{Key: "c"},
		{Key: "d"},
	}

	t.Run("no filter returns all", func(t *testing.T) {
		result := FilterByHash(candidates, HashFilterNone)
		if len(result) != len(candidates) {
			t.Errorf("got %d candidates, want %d", len(result), len(candidates))
		}
	})

	t.Run("evens and odds are disjoint and complete", func(t *testing.T) {
		evens := FilterByHash(candidates, HashFilterEvens)
		odds := FilterByHash(candidates, HashFilterOdds)

		// Together they should cover all candidates
		if len(evens)+len(odds) != len(candidates) {
			t.Errorf("evens (%d) + odds (%d) != total (%d)", len(evens), len(odds), len(candidates))
		}

		// They should be disjoint
		evenKeys := make(map[string]bool)
		for _, c := range evens {
			evenKeys[c.Key] = true
		}
		for _, c := range odds {
			if evenKeys[c.Key] {
				t.Errorf("key %q appears in both evens and odds", c.Key)
			}
		}
	})
}

func TestIgnoredList(t *testing.T) {
	t.Run("contains works correctly", func(t *testing.T) {
		dir := t.TempDir()
		ignoredPath := filepath.Join(dir, "ignored.log")

		// Create ignored.log with some entries
		err := os.WriteFile(ignoredPath, []byte("file1.go\nfile2.go\n"), 0644)
		if err != nil {
			t.Fatalf("failed to create ignored.log: %v", err)
		}

		list, err := NewIgnoredList(dir)
		if err != nil {
			t.Fatalf("NewIgnoredList failed: %v", err)
		}

		if !list.Contains("file1.go") {
			t.Error("expected file1.go to be ignored")
		}
		if !list.Contains("file2.go") {
			t.Error("expected file2.go to be ignored")
		}
		if list.Contains("file3.go") {
			t.Error("expected file3.go to not be ignored")
		}
	})

	t.Run("add appends to file", func(t *testing.T) {
		dir := t.TempDir()

		list, err := NewIgnoredList(dir)
		if err != nil {
			t.Fatalf("NewIgnoredList failed: %v", err)
		}

		if err := list.Add("newfile.go"); err != nil {
			t.Fatalf("Add failed: %v", err)
		}

		if !list.Contains("newfile.go") {
			t.Error("expected newfile.go to be ignored after adding")
		}

		// Verify file was written
		content, err := os.ReadFile(filepath.Join(dir, "ignored.log"))
		if err != nil {
			t.Fatalf("failed to read ignored.log: %v", err)
		}
		if string(content) != "newfile.go\n" {
			t.Errorf("file content = %q, want %q", string(content), "newfile.go\n")
		}
	})

	t.Run("empty directory creates new list", func(t *testing.T) {
		dir := t.TempDir()

		list, err := NewIgnoredList(dir)
		if err != nil {
			t.Fatalf("NewIgnoredList failed: %v", err)
		}

		if list.Contains("anything") {
			t.Error("new list should not contain any entries")
		}
	})
}

func TestDeterministicMapKeys(t *testing.T) {
	t.Run("map keys are deterministic across parses", func(t *testing.T) {
		// Parse the same JSON multiple times
		jsonInput := `[{"file": "test.go", "line": 10}, {"line": 20, "file": "other.go"}]`

		var keys1 []string
		for i := 0; i < 10; i++ {
			candidates, err := ParseCandidates([]byte(jsonInput))
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			var currentKeys []string
			for _, c := range candidates {
				currentKeys = append(currentKeys, c.Key)
			}

			if i == 0 {
				keys1 = currentKeys
			} else {
				// Keys should be identical across all parses
				for j, k := range currentKeys {
					if k != keys1[j] {
						t.Errorf("parse %d: key[%d] = %q, want %q", i, j, k, keys1[j])
					}
				}
			}
		}
	})

	t.Run("map keys are sorted for consistency", func(t *testing.T) {
		// Regardless of input order, output should be sorted
		input1 := `[{"file": "test.go", "line": 10}]`
		input2 := `[{"line": 10, "file": "test.go"}]`

		candidates1, _ := ParseCandidates([]byte(input1))
		candidates2, _ := ParseCandidates([]byte(input2))

		if len(candidates1) != 1 || len(candidates2) != 1 {
			t.Fatal("expected 1 candidate each")
		}

		if candidates1[0].Key != candidates2[0].Key {
			t.Errorf("keys differ: %q vs %q", candidates1[0].Key, candidates2[0].Key)
		}

		// Verify keys are alphabetically sorted
		expected := `{"file":"test.go","line":10}`
		if candidates1[0].Key != expected {
			t.Errorf("key = %q, want %q (sorted)", candidates1[0].Key, expected)
		}
	})
}

func TestSelectCandidate(t *testing.T) {
	t.Run("selects first non-ignored candidate", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "ignored.log"), []byte("file1.go\nfile2.go\n"), 0644)
		if err != nil {
			t.Fatalf("failed to create ignored.log: %v", err)
		}

		list, _ := NewIgnoredList(dir)
		candidates := []Candidate{
			{Key: "file1.go"},
			{Key: "file2.go"},
			{Key: "file3.go"},
			{Key: "file4.go"},
		}

		result := SelectCandidate(candidates, list)
		if result == nil {
			t.Fatal("expected a candidate to be selected")
		}
		if result.Key != "file3.go" {
			t.Errorf("selected %q, want %q", result.Key, "file3.go")
		}
	})

	t.Run("returns nil when all ignored", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "ignored.log"), []byte("file1.go\nfile2.go\n"), 0644)
		if err != nil {
			t.Fatalf("failed to create ignored.log: %v", err)
		}

		list, _ := NewIgnoredList(dir)
		candidates := []Candidate{
			{Key: "file1.go"},
			{Key: "file2.go"},
		}

		result := SelectCandidate(candidates, list)
		if result != nil {
			t.Errorf("expected nil, got %q", result.Key)
		}
	})

	t.Run("counts ignored candidates correctly", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "ignored.log"), []byte("a\nb\nc\n"), 0644)
		if err != nil {
			t.Fatalf("failed to create ignored.log: %v", err)
		}

		list, _ := NewIgnoredList(dir)
		candidates := []Candidate{
			{Key: "a"},
			{Key: "b"},
			{Key: "c"},
			{Key: "d"},
			{Key: "e"},
		}

		// Count ignored (same logic as runner.go)
		ignoredCount := 0
		for _, c := range candidates {
			if list.Contains(c.Key) {
				ignoredCount++
			}
		}

		if ignoredCount != 3 {
			t.Errorf("ignoredCount = %d, want 3", ignoredCount)
		}
		if len(candidates)-ignoredCount != 2 {
			t.Errorf("available = %d, want 2", len(candidates)-ignoredCount)
		}
	})
}
