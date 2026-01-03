package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// RunCandidateSource executes a candidate source command and returns its stdout.
func RunCandidateSource(source, workDir string) ([]byte, error) {
	cmd := exec.Command("bash", "-c", source)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("candidate source failed: %w\nstderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// RunCommand executes a shell command and returns success status.
func RunCommand(command, workDir string) (bool, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RunCommandSilent executes a shell command without output and returns success status.
func RunCommandSilent(command, workDir string) (bool, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = workDir

	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// RunClaudeCommand executes the Claude command with prompt, streaming output to both stdout and a log writer.
// Returns the captured output (for rate limit detection) and any error.
func RunClaudeCommand(claudeCmd, claudeFlags, prompt, workDir string, logWriter io.Writer) (string, error) {
	// Build the command
	args := []string{"-c"}

	// Escape prompt for shell
	escapedPrompt := shellQuote(prompt)

	var cmdStr string
	if claudeFlags != "" {
		cmdStr = fmt.Sprintf("%s %s -p %s", claudeCmd, claudeFlags, escapedPrompt)
	} else {
		cmdStr = fmt.Sprintf("%s -p %s", claudeCmd, escapedPrompt)
	}

	args = append(args, cmdStr)

	cmd := exec.Command("bash", args...)
	cmd.Dir = workDir

	// Buffer to capture output for rate limit detection
	var outputBuf bytes.Buffer

	// Create a multi-writer to tee output to stdout, log, and capture buffer
	var multiOut, multiErr io.Writer
	if logWriter != nil {
		multiOut = io.MultiWriter(os.Stdout, logWriter, &outputBuf)
		multiErr = io.MultiWriter(os.Stderr, logWriter, &outputBuf)
	} else {
		multiOut = io.MultiWriter(os.Stdout, &outputBuf)
		multiErr = io.MultiWriter(os.Stderr, &outputBuf)
	}

	cmd.Stdout = multiOut
	cmd.Stderr = multiErr

	err := cmd.Run()
	return outputBuf.String(), err
}

// shellQuote returns a shell-safe quoted string.
func shellQuote(s string) string {
	// Use single quotes, escaping any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

// Regex patterns for $INPUT interpolation
var (
	// $INPUT["key"] - map key access
	inputMapKeyRe = regexp.MustCompile(`\$INPUT\["([^"]+)"\]`)
	// $INPUT[n:] - slice from index
	inputSliceRe = regexp.MustCompile(`\$INPUT\[(\d+):\]`)
	// $INPUT[n] - array index access
	inputIndexRe = regexp.MustCompile(`\$INPUT\[(\d+)\]`)
	// $INPUT - bare input (must be checked last)
	inputBareRe = regexp.MustCompile(`\$INPUT\b`)
)

// InterpolatePrompt replaces template variables with candidate values.
// Supports: $INPUT, $INPUT[n], $INPUT[n:], $INPUT["key"]
func InterpolatePrompt(template string, candidate *Candidate) string {
	result := template

	// Replace $INPUT["key"] - map key access
	result = inputMapKeyRe.ReplaceAllStringFunc(result, func(match string) string {
		submatch := inputMapKeyRe.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		key := submatch[1]
		if val, ok := candidate.GetKey(key); ok {
			return val
		}
		return ""
	})

	// Replace $INPUT[n:] - slice from index
	result = inputSliceRe.ReplaceAllStringFunc(result, func(match string) string {
		submatch := inputSliceRe.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		idx, _ := strconv.Atoi(submatch[1])
		if val, ok := candidate.GetSlice(idx); ok {
			return val
		}
		return "[]"
	})

	// Replace $INPUT[n] - array index access
	result = inputIndexRe.ReplaceAllStringFunc(result, func(match string) string {
		submatch := inputIndexRe.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		idx, _ := strconv.Atoi(submatch[1])
		if val, ok := candidate.GetIndex(idx); ok {
			return val
		}
		return ""
	})

	// Replace bare $INPUT - whole value (with single-item unwrap)
	result = inputBareRe.ReplaceAllStringFunc(result, func(match string) string {
		return candidate.String()
	})

	return result
}

// InterpolateCommand replaces template variables in commands.
// Supports: $CANDIDATE, $TASK_NAME
func InterpolateCommand(command string, candidate *Candidate, taskName string) string {
	result := strings.ReplaceAll(command, "$CANDIDATE", candidate.Key)
	result = strings.ReplaceAll(result, "$TASK_NAME", taskName)
	return result
}

// LoadTemplate reads a template file and returns its contents.
func LoadTemplate(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read template: %w", err)
	}
	return string(data), nil
}

// HasUncommittedChanges checks if there are uncommitted git changes.
func HasUncommittedChanges(workDir string) (bool, error) {
	cmd := exec.Command("git", "diff", "--quiet")
	cmd.Dir = workDir
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return true, nil
		}
		return false, err
	}

	// Also check staged changes
	cmd = exec.Command("git", "diff", "--quiet", "--cached")
	cmd.Dir = workDir
	err = cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return true, nil
		}
		return false, err
	}

	// Also check untracked files
	cmd = exec.Command("git", "status", "--porcelain")
	cmd.Dir = workDir
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return len(strings.TrimSpace(string(output))) > 0, nil
}

// CheckClaudeCommand verifies the Claude command is accessible.
func CheckClaudeCommand(claudeCmd string) error {
	// Extract just the command name (first part before any spaces)
	parts := strings.Fields(claudeCmd)
	if len(parts) == 0 {
		return fmt.Errorf("empty claude command")
	}

	_, err := exec.LookPath(parts[0])
	if err != nil {
		return fmt.Errorf("claude command not found: %s", parts[0])
	}
	return nil
}

// parseInt parses a string to int, returning 0 on error.
func parseInt(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}
