package main

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

// CommandExecutor executes shell commands.
type CommandExecutor interface {
	// Run executes a command with output to stdout/stderr.
	Run(command, workDir string) (bool, error)

	// RunSilent executes a command without output.
	RunSilent(command, workDir string) (bool, error)

	// RunShowOnFail executes a command, showing output only on failure.
	RunShowOnFail(command, workDir string) (bool, error)

	// HasUncommittedChanges checks if there are uncommitted git changes.
	HasUncommittedChanges(workDir string) (bool, error)
}

// RealCommandExecutor executes actual shell commands.
type RealCommandExecutor struct{}

// Run executes a shell command and returns success status.
func (r *RealCommandExecutor) Run(command, workDir string) (bool, error) {
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

// RunSilent executes a shell command without output and returns success status.
func (r *RealCommandExecutor) RunSilent(command, workDir string) (bool, error) {
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

// RunShowOnFail executes a shell command, capturing output and only printing it if the command fails.
func (r *RealCommandExecutor) RunShowOnFail(command, workDir string) (bool, error) {
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

// HasUncommittedChanges checks if there are uncommitted git changes.
func (r *RealCommandExecutor) HasUncommittedChanges(workDir string) (bool, error) {
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

// RunCommand is a convenience function that uses RealCommandExecutor.
// Kept for backward compatibility.
func RunCommand(command, workDir string) (bool, error) {
	return (&RealCommandExecutor{}).Run(command, workDir)
}

// RunCommandSilent is a convenience function that uses RealCommandExecutor.
// Kept for backward compatibility.
func RunCommandSilent(command, workDir string) (bool, error) {
	return (&RealCommandExecutor{}).RunSilent(command, workDir)
}

// RunCommandShowOnFail is a convenience function that uses RealCommandExecutor.
// Kept for backward compatibility.
func RunCommandShowOnFail(command, workDir string) (bool, error) {
	return (&RealCommandExecutor{}).RunShowOnFail(command, workDir)
}

// HasUncommittedChanges is a convenience function that uses RealCommandExecutor.
// Kept for backward compatibility.
func HasUncommittedChanges(workDir string) (bool, error) {
	return (&RealCommandExecutor{}).HasUncommittedChanges(workDir)
}
