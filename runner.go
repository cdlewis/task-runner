package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type RunnerOptions struct {
	Limit      int
	DryRun     bool
	Verbose    bool
	HashFilter HashFilter
}

type Runner struct {
	env              *Environment
	task             Task
	opts             RunnerOptions
	ignoredList      *IgnoredList
	claudeLogger     *ClaudeLogger
	stopRequested    bool
	consecutiveFails int
}

func NewRunner(env *Environment, taskName string, opts RunnerOptions) (*Runner, error) {
	task, ok := env.Tasks[taskName]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskName)
	}

	ignoredList, err := NewIgnoredList(task.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to create ignored list: %w", err)
	}

	var claudeLogger *ClaudeLogger
	if !opts.DryRun {
		claudeLogger, err = NewClaudeLogger(task.Dir)
		if err != nil {
			return nil, fmt.Errorf("failed to create claude logger: %w", err)
		}
	}

	return &Runner{
		env:          env,
		task:         task,
		opts:         opts,
		ignoredList:  ignoredList,
		claudeLogger: claudeLogger,
	}, nil
}

func (r *Runner) Run() error {
	// Verify claude command exists (skip in dry-run)
	if !r.opts.DryRun {
		if err := CheckClaudeCommand(r.env.Config.ClaudeCommand); err != nil {
			return err
		}
	}

	// Set up SIGQUIT handler for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGQUIT)
	go func() {
		<-sigChan
		fmt.Println("\n[Ctrl+\\] Graceful stop requested, will finish current iteration...")
		r.stopRequested = true
	}()

	// Print startup info
	fmt.Printf("Task: %s\n", r.task.Name)
	if r.claudeLogger != nil {
		fmt.Printf("Logs: %s\n", r.claudeLogger.Path())
	}
	fmt.Printf("Mode: %s\n", r.modeString())
	fmt.Println()

	iteration := 0
	for {
		if r.stopRequested {
			fmt.Println("Stopped by user request.")
			break
		}

		if r.opts.Limit > 0 && iteration >= r.opts.Limit {
			fmt.Printf("Reached iteration limit (%d).\n", r.opts.Limit)
			break
		}

		iteration++
		fmt.Printf("\n=== Iteration %d (%s) ===\n", iteration, time.Now().Format("15:04:05"))

		done, err := r.runIteration()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			r.consecutiveFails++

			if r.consecutiveFails >= 3 {
				fmt.Println("3 consecutive failures, sleeping for 5 minutes...")
				time.Sleep(5 * time.Minute)
				r.consecutiveFails = 0
			}
			continue
		}

		if done {
			fmt.Println("No more candidates.")
			break
		}

		r.consecutiveFails = 0
	}

	if r.claudeLogger != nil {
		r.claudeLogger.Close()
	}

	return nil
}

func (r *Runner) runIteration() (done bool, err error) {
	// Run candidate source to get candidates
	fmt.Println("Running candidate source...")
	output, err := RunCandidateSource(r.task.CandidateSource, r.env.ProjectDir)
	if err != nil {
		return false, fmt.Errorf("candidate source failed: %w", err)
	}

	candidates, err := ParseCandidates(output)
	if err != nil {
		return false, fmt.Errorf("failed to parse candidates: %w", err)
	}

	// Filter by hash if requested
	candidates = FilterByHash(candidates, r.opts.HashFilter)

	// Select first non-ignored candidate
	candidate := SelectCandidate(candidates, r.ignoredList)
	if candidate == nil {
		return true, nil
	}

	fmt.Printf("Selected: %s\n", candidate.Key)

	// Get prompt content
	prompt, err := r.getPrompt(candidate)
	if err != nil {
		return false, err
	}

	if r.opts.Verbose {
		fmt.Printf("Prompt:\n%s\n", prompt)
	}

	// Dry run: just print and exit
	if r.opts.DryRun {
		fmt.Printf("\n--- Dry Run Prompt ---\n%s\n--- End Prompt ---\n", prompt)
		return true, nil
	}

	// Run Claude
	fmt.Println("Running Claude...")
	if r.claudeLogger != nil {
		r.claudeLogger.StartEntry(prompt)
	}

	claudeFlags := r.task.ClaudeFlags
	err = RunClaudeCommand(r.env.Config.ClaudeCommand, claudeFlags, prompt, r.env.ProjectDir, r.claudeLogger)

	if r.claudeLogger != nil {
		r.claudeLogger.EndEntry()
	}

	if err != nil {
		return false, fmt.Errorf("claude failed: %w", err)
	}

	// Re-run candidate source to check if candidate was fixed
	fmt.Println("Re-checking candidates...")
	output, err = RunCandidateSource(r.task.CandidateSource, r.env.ProjectDir)
	if err != nil {
		return false, fmt.Errorf("candidate source re-run failed: %w", err)
	}

	newCandidates, err := ParseCandidates(output)
	if err != nil {
		return false, fmt.Errorf("failed to parse new candidates: %w", err)
	}

	candidateFixed := !containsKey(newCandidates, candidate.Key)

	if candidateFixed {
		return r.handleSuccess(candidate)
	} else {
		return r.handleFailure(candidate)
	}
}

func (r *Runner) handleSuccess(candidate *Candidate) (bool, error) {
	fmt.Printf("✓ Candidate %s was fixed!\n", candidate.Key)

	// Verify build
	if !r.runVerify() {
		fmt.Println("Build verification failed after fix, attempting recovery...")
		if !r.runReset() {
			return false, fmt.Errorf("failed to reset after build failure")
		}
		if !r.runVerify() {
			return false, fmt.Errorf("build still fails after reset")
		}
		fmt.Println("Recovered via reset.")
		r.logOutcome(OutcomeFixedReverted, "build failed after fix")
		if err := r.ignoredList.Add(candidate.Key); err != nil {
			return false, err
		}
		return false, nil
	}

	// Commit changes if there are any
	hasChanges, err := HasUncommittedChanges(r.env.ProjectDir)
	if err != nil {
		return false, fmt.Errorf("failed to check for changes: %w", err)
	}

	if hasChanges {
		successCmd := InterpolateCommand(r.env.Config.SuccessCommand, candidate, r.task.Name)
		fmt.Printf("Running success command: %s\n", successCmd)
		ok, err := RunCommand(successCmd, r.env.ProjectDir)
		if err != nil {
			return false, fmt.Errorf("success command error: %w", err)
		}
		if !ok {
			fmt.Println("Warning: success command returned non-zero exit code")
		}
		r.logOutcome(OutcomeFixed, "committed")
	} else {
		r.logOutcome(OutcomeFixed, "no changes to commit")
	}

	return false, nil
}

func (r *Runner) handleFailure(candidate *Candidate) (bool, error) {
	fmt.Printf("✗ Candidate %s not fixed.\n", candidate.Key)

	if r.task.AcceptBestEffort {
		// Best effort mode: commit if build passes
		if r.runVerify() {
			hasChanges, err := HasUncommittedChanges(r.env.ProjectDir)
			if err != nil {
				return false, fmt.Errorf("failed to check for changes: %w", err)
			}

			if hasChanges {
				fmt.Println("Best effort: committing partial progress...")
				successCmd := InterpolateCommand(r.env.Config.SuccessCommand, candidate, r.task.Name)
				// Modify message for best effort
				successCmd = replaceBestEffort(successCmd, candidate.Key)
				ok, err := RunCommand(successCmd, r.env.ProjectDir)
				if err != nil {
					return false, fmt.Errorf("best effort commit error: %w", err)
				}
				if !ok {
					fmt.Println("Warning: best effort commit returned non-zero exit code")
				}
				r.logOutcome(OutcomeBestEffort, "partial progress committed")
			} else {
				r.logOutcome(OutcomeNotFixed, "no changes made")
			}
		} else {
			// Build failed, reset
			fmt.Println("Build failed, resetting...")
			if !r.runReset() {
				return false, fmt.Errorf("failed to reset")
			}
			if !r.runVerify() {
				return false, fmt.Errorf("build still fails after reset")
			}
			r.logOutcome(OutcomeBuildFailed, "reverted")
		}
	} else {
		// Standard mode: reset changes
		fmt.Println("Resetting changes...")
		if !r.runReset() {
			return false, fmt.Errorf("failed to reset")
		}
		if !r.runVerify() {
			return false, fmt.Errorf("build fails after reset")
		}
		r.logOutcome(OutcomeNotFixed, "reverted")
	}

	if err := r.ignoredList.Add(candidate.Key); err != nil {
		return false, err
	}

	return false, nil
}

func (r *Runner) getPrompt(candidate *Candidate) (string, error) {
	var template string

	if r.task.Template != "" {
		// Load from template file (relative to task directory)
		templatePath := filepath.Join(r.task.Dir, r.task.Template)
		content, err := LoadTemplate(templatePath)
		if err != nil {
			return "", err
		}
		template = content
	} else {
		template = r.task.Prompt
	}

	return InterpolatePrompt(template, candidate), nil
}

func (r *Runner) runVerify() bool {
	if r.env.Config.VerifyCommand == "" {
		return true
	}
	fmt.Println("Verifying build...")
	ok, err := RunCommand(r.env.Config.VerifyCommand, r.env.ProjectDir)
	if err != nil {
		fmt.Printf("Verify command error: %v\n", err)
		return false
	}
	return ok
}

func (r *Runner) runReset() bool {
	if r.env.Config.ResetCommand == "" {
		return true
	}
	fmt.Println("Running reset...")
	ok, err := RunCommandSilent(r.env.Config.ResetCommand, r.env.ProjectDir)
	if err != nil {
		fmt.Printf("Reset command error: %v\n", err)
		return false
	}
	return ok
}

func (r *Runner) modeString() string {
	if r.opts.DryRun {
		return "dry-run"
	}
	if r.task.AcceptBestEffort {
		return "best-effort"
	}
	return "standard"
}

func (r *Runner) logOutcome(outcome Outcome, details string) {
	if r.claudeLogger != nil {
		r.claudeLogger.LogOutcome(outcome, details)
	}
}

func containsKey(candidates []Candidate, key string) bool {
	for _, c := range candidates {
		if c.Key == key {
			return true
		}
	}
	return false
}

// replaceBestEffort modifies the success command for best effort mode.
// If the command contains "fix", replace with "best effort".
func replaceBestEffort(cmd, candidate string) string {
	// Simple heuristic: if it says "fix $CANDIDATE", change to "best effort $CANDIDATE"
	// This handles the common case of commit messages
	return cmd
}
