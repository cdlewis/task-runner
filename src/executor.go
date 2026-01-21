package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// timeoutError indicates Claude execution timed out
type timeoutError struct {
	duration time.Duration
}

// StreamCallback is called for each chunk of text received from Claude.
type StreamCallback func(text string)

// Claude stream event types
type streamEvent struct {
	Type  string                 `json:"type"`
	Event map[string]interface{} `json:"event,omitempty"`
}

// contentBlockDelta represents the delta content in a stream event
type contentBlockDelta struct {
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta"`
}

// resultEvent represents the final result event
type resultEvent struct {
	Type   string `json:"type"`
	Result string `json:"result,omitempty"`
}

func (e *timeoutError) Error() string {
	return fmt.Sprintf("timeout after %s", e.duration)
}

func (e *timeoutError) IsTimeout() bool {
	return true
}

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

// RunCommandShowOnFail executes a shell command, capturing output and only printing it if the command fails.
func RunCommandShowOnFail(command, workDir string) (bool, error) {
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Command failed - print captured output
			if stdout.Len() > 0 {
				os.Stdout.Write(stdout.Bytes())
			}
			if stderr.Len() > 0 {
				os.Stderr.Write(stderr.Bytes())
			}
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// runningProcess tracks the currently running Claude process for signal forwarding
var runningProcess *os.Process

// KillRunningProcess terminates the running Claude process if any
func KillRunningProcess() {
	if p := runningProcess; p != nil {
		// Kill the entire process group
		syscall.Kill(-p.Pid, syscall.SIGTERM)
	}
}

// RunClaudeCommand executes the Claude command with prompt, timeout, and streaming output.
// The streamCb callback is invoked for each chunk of text received.
// Returns the accumulated output (for rate limit detection) and any error.
func RunClaudeCommand(claudeCmd, claudeFlags, prompt, workDir string, logWriter io.Writer, timeout time.Duration, streamCb StreamCallback) (string, error) {
	// Build the command using heredoc to avoid shell escaping issues
	// Using --output-format stream-json --include-partial-messages --verbose
	// Note: --print is required for --output-format to work
	const delimiter = "__NIGEL_PROMPT_EOF__"
	jsonFlags := "--print --output-format stream-json --include-partial-messages --verbose"

	var cmdStr string
	if claudeFlags != "" {
		cmdStr = fmt.Sprintf("%s %s %s -p <<'%s'\n%s\n%s",
			claudeCmd, jsonFlags, claudeFlags, delimiter, prompt, delimiter)
	} else {
		cmdStr = fmt.Sprintf("%s %s -p <<'%s'\n%s\n%s",
			claudeCmd, jsonFlags, delimiter, prompt, delimiter)
	}

	// Log the exact command being executed (for debugging hangs)
	if logWriter != nil {
		fmt.Fprintf(logWriter, "Command: %s\n", cmdStr)
	}

	args := []string{"-c", cmdStr}

	cmd := exec.Command("bash", args...)
	cmd.Dir = workDir
	// Put child in its own process group so it doesn't receive SIGQUIT.
	// Pdeathsig ensures child is killed if parent dies unexpectedly (Linux only).
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid:   true,
		Pdeathsig: syscall.SIGTERM,
	}

	// Create pipe for stdout so we can read line-by-line
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	// Capture stderr to buffer
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	// Start the process and track it for signal forwarding
	if err := cmd.Start(); err != nil {
		return "", err
	}
	runningProcess = cmd.Process

	// Goroutine to read stdout line-by-line and parse JSON
	type streamResult struct {
		fullOutput string
		err        error
	}
	resultCh := make(chan streamResult, 1)

	go func() {
		var fullOutput strings.Builder
		var messageHasContent bool
		scanner := bufio.NewScanner(stdoutPipe)
		// Increase buffer size to handle large JSON responses from Claude
		// Default is 64KB which isn't enough for large code blocks
		scanner.Buffer(nil, 10*1024*1024) // 10MB max token size

		for scanner.Scan() {
			line := scanner.Text()

			// Try to parse as stream event
			var se streamEvent
			if jsonErr := json.Unmarshal([]byte(line), &se); jsonErr != nil {
				// Not valid JSON - write as-is to log and continue
				if logWriter != nil {
					fmt.Fprintln(logWriter, line)
				}
				fullOutput.WriteString(line + "\n")
				continue
			}

			// Handle different event types
			switch se.Type {
			case "stream_event":
				// Check if this is a content_block_delta
				if eventType, ok := se.Event["type"].(string); ok && eventType == "content_block_delta" {
					// Extract the delta text
					eventJSON, _ := json.Marshal(se.Event)
					var delta contentBlockDelta
					if json.Unmarshal(eventJSON, &delta) == nil && delta.Delta.Type == "text_delta" && delta.Delta.Text != "" {
						text := delta.Delta.Text
						messageHasContent = true
						// Stream the text content to stdout
						if streamCb != nil {
							streamCb(text)
						}
						// Also write to log
						if logWriter != nil {
							fmt.Fprint(logWriter, text)
						}
						fullOutput.WriteString(text)
					}
				}
				// Check if this is message_stop - add newline between messages (only if content was received)
				if eventType, ok := se.Event["type"].(string); ok && eventType == "message_stop" {
					if messageHasContent {
						if streamCb != nil {
							streamCb("\n")
						}
						if logWriter != nil {
							fmt.Fprint(logWriter, "\n")
						}
						fullOutput.WriteString("\n")
					}
					messageHasContent = false
				}

			case "result":
				// Final result event - completion confirmed
				var re resultEvent
				if json.Unmarshal([]byte(line), &re) == nil && re.Result != "" {
					// Completion confirmed
				}
			}
		}

		// Add a final newline after streaming is complete
		if streamCb != nil {
			streamCb("\n")
		}
		if logWriter != nil {
			fmt.Fprintln(logWriter)
		}

		// Include stderr in output for rate limit detection
		if stderrBuf.Len() > 0 {
			fullOutput.WriteString(stderrBuf.String())
		}

		resultCh <- streamResult{
			fullOutput: fullOutput.String(),
			err:        scanner.Err(),
		}
	}()

	// Wait for completion or timeout
	var waitErr error
	if timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-time.After(timeout):
			KillRunningProcess()
			runningProcess = nil
			// Wait for the stream reader to finish
			result := <-resultCh
			return result.fullOutput, &timeoutError{duration: timeout}
		case waitErr = <-done:
			runningProcess = nil
		}
	} else {
		waitErr = cmd.Wait()
		runningProcess = nil
	}

	// Get the full output from the stream reader
	result := <-resultCh
	if result.err != nil {
		return result.fullOutput, result.err
	}

	return result.fullOutput, waitErr
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
// Supports: $INPUT, $INPUT[n], $INPUT[n:], $INPUT["key"], $TASK_ID
func InterpolatePrompt(template string, candidate *Candidate, taskID int64) string {
	result := template

	// Replace $TASK_ID - unique task identifier
	result = strings.ReplaceAll(result, "$TASK_ID", fmt.Sprintf("%d", taskID))

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

// shellQuote wraps a value in single quotes for safe shell interpolation.
// Single quotes within the value are handled by ending the quote, adding an escaped quote, and restarting.
// Example: O'Reilly -> 'O'"'"'Reilly'
func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	// Single quotes make everything literal, except single quotes themselves.
	// To handle single quotes in the value, we exit the single-quote context,
	// add an escaped double-quote, and re-enter single-quote context.
	// 'value' -> 'value'
	// O'Reilly -> 'O'"'"'Reilly'
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// InterpolateCommand replaces template variables in commands.
// Supports: $CANDIDATE, $TASK_NAME
// $CANDIDATE is shell-quoted to safely handle special characters.
func InterpolateCommand(command string, candidate *Candidate, taskName string) string {
	result := strings.ReplaceAll(command, "$CANDIDATE", shellQuote(candidate.Key))
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
