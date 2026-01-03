package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
)

// Candidate represents a work item from the candidate source output.
// It can be a string, array, or map - stored as raw JSON for flexible access.
type Candidate struct {
	Key  string          // JSON serialization of the full candidate (for uniqueness)
	Data json.RawMessage // Raw JSON data (string, array, or map)
}

type HashFilter int

const (
	HashFilterNone HashFilter = iota
	HashFilterEvens
	HashFilterOdds
)

// ParseCandidates parses the JSON output from a candidate source.
// Supports: ["a", "b"], [["a", "x"], ["b", "y"]], or [{"file": "a"}, {"file": "b"}]
func ParseCandidates(jsonData []byte) ([]Candidate, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(jsonData, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse JSON array: %w", err)
	}

	candidates := make([]Candidate, 0, len(raw))
	for _, item := range raw {
		// Compact the JSON for consistent key generation
		var buf bytes.Buffer
		if err := json.Compact(&buf, item); err != nil {
			return nil, fmt.Errorf("failed to compact JSON: %w", err)
		}

		key := buf.String()

		// For simple strings, use the unquoted value as the key
		var str string
		if err := json.Unmarshal(item, &str); err == nil {
			key = str
		}

		candidates = append(candidates, Candidate{
			Key:  key,
			Data: item,
		})
	}

	return candidates, nil
}

// IsArray returns true if the candidate data is a JSON array.
func (c *Candidate) IsArray() bool {
	return len(c.Data) > 0 && c.Data[0] == '['
}

// IsMap returns true if the candidate data is a JSON object.
func (c *Candidate) IsMap() bool {
	return len(c.Data) > 0 && c.Data[0] == '{'
}

// IsString returns true if the candidate data is a JSON string.
func (c *Candidate) IsString() bool {
	return len(c.Data) > 0 && c.Data[0] == '"'
}

// GetIndex returns the element at the given index (0-based) for array candidates.
// Returns the value as a string (JSON-serialized if not a string type).
func (c *Candidate) GetIndex(i int) (string, bool) {
	if !c.IsArray() {
		return "", false
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(c.Data, &arr); err != nil {
		return "", false
	}

	if i < 0 || i >= len(arr) {
		return "", false
	}

	return rawToString(arr[i]), true
}

// GetSlice returns elements from the given index to the end as a JSON array.
func (c *Candidate) GetSlice(start int) (string, bool) {
	if !c.IsArray() {
		return "", false
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(c.Data, &arr); err != nil {
		return "", false
	}

	if start < 0 || start >= len(arr) {
		return "[]", true
	}

	slice := arr[start:]
	result, err := json.Marshal(slice)
	if err != nil {
		return "", false
	}

	return string(result), true
}

// GetKey returns the value for the given key in a map candidate.
// Returns the value as a string (JSON-serialized if not a string type).
func (c *Candidate) GetKey(key string) (string, bool) {
	if !c.IsMap() {
		return "", false
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(c.Data, &m); err != nil {
		return "", false
	}

	val, ok := m[key]
	if !ok {
		return "", false
	}

	return rawToString(val), true
}

// String returns the candidate data as a string.
// Single-item arrays are unwrapped for convenience.
func (c *Candidate) String() string {
	// For strings, return the unquoted value
	if c.IsString() {
		var str string
		if err := json.Unmarshal(c.Data, &str); err == nil {
			return str
		}
	}

	// For single-item arrays, unwrap and return the item
	if c.IsArray() {
		var arr []json.RawMessage
		if err := json.Unmarshal(c.Data, &arr); err == nil && len(arr) == 1 {
			return rawToString(arr[0])
		}
	}

	// Otherwise return the full JSON
	return string(c.Data)
}

// rawToString converts a json.RawMessage to a string.
// If it's a JSON string, returns the unquoted value.
// Otherwise returns the JSON representation.
func rawToString(raw json.RawMessage) string {
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		return str
	}
	return string(raw)
}

// FilterByHash filters candidates by MD5 hash parity.
func FilterByHash(candidates []Candidate, filter HashFilter) []Candidate {
	if filter == HashFilterNone {
		return candidates
	}

	filtered := make([]Candidate, 0)
	for _, c := range candidates {
		hash := md5.Sum([]byte(c.Key))
		hashInt := new(big.Int).SetBytes(hash[:])
		isEven := hashInt.Bit(0) == 0

		if (filter == HashFilterEvens && isEven) || (filter == HashFilterOdds && !isEven) {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

// IgnoredList manages the list of already-processed candidates.
type IgnoredList struct {
	path    string
	entries map[string]bool
}

func NewIgnoredList(taskDir string) (*IgnoredList, error) {
	path := filepath.Join(taskDir, "ignored.log")
	entries := make(map[string]bool)

	file, err := os.Open(path)
	if err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				entries[line] = true
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read ignored list: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to open ignored list: %w", err)
	}

	return &IgnoredList{
		path:    path,
		entries: entries,
	}, nil
}

func (l *IgnoredList) Contains(key string) bool {
	return l.entries[key]
}

func (l *IgnoredList) Add(key string) error {
	if l.entries[key] {
		return nil
	}

	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open ignored list for writing: %w", err)
	}
	defer file.Close()

	if _, err := fmt.Fprintln(file, key); err != nil {
		return fmt.Errorf("failed to write to ignored list: %w", err)
	}

	l.entries[key] = true
	return nil
}

// SelectCandidate returns the first candidate not in the ignored list.
func SelectCandidate(candidates []Candidate, ignored *IgnoredList) *Candidate {
	for _, c := range candidates {
		if !ignored.Contains(c.Key) {
			return &c
		}
	}
	return nil
}
