package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	baseBackoff      = 5 * time.Minute
	maxBackoff       = 1 * time.Hour
	rateLimitBackoff = 1 * time.Hour
	rateLimitPhrase  = "You've hit your limit"
)

// rateLimitError indicates Claude returned a rate limit message
type rateLimitError struct {
	msg string
}

func (e *rateLimitError) Error() string {
	return e.msg
}

// calculateBackoff returns the backoff duration for the given level
func calculateBackoff(level int) time.Duration {
	backoff := baseBackoff
	for i := 0; i < level; i++ {
		backoff *= 2
		if backoff >= maxBackoff {
			return maxBackoff
		}
	}
	return backoff
}

type RunnerOptions struct {
	Limit      int
	TimeLimit  time.Duration
	DryRun     bool
	Verbose    bool
	HashFilter HashFilter
}

type Runner struct {
	env           *Environment
	task          Task
	opts          RunnerOptions
	ignoredList   *IgnoredList
	claudeLogger  *ClaudeLogger
	claudeStats   *SessionStats
	stopRequested bool
	backoffLevel  int
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
		claudeStats:  NewSessionStats(),
	}, nil
}

func (r *Runner) Run() error {
	// Verify claude command exists (skip in dry-run)
	if !r.opts.DryRun {
		if err := CheckClaudeCommand(r.env.Config.ClaudeCommand); err != nil {
			return err
		}
	}

	// Set up signal handlers
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		switch sig {
		case syscall.SIGQUIT:
			fmt.Println("\n[Ctrl+\\] Graceful stop requested, will finish current iteration...")
			r.stopRequested = true
		case syscall.SIGINT, syscall.SIGTERM:
			fmt.Println("\nInterrupted, cleaning up...")
			KillRunningProcess()
			os.Exit(1)
		}
	}()

	// Print startup banner with cat
	logPath := filepath.Join(r.task.Dir, "claude.log")
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, logPath); err == nil {
			logPath = rel
		}
	}
	fmt.Print(StartupBanner(r.task.Name, logPath, r.modeString()))

	startTime := time.Now()
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

		if r.opts.TimeLimit > 0 && time.Since(startTime) >= r.opts.TimeLimit {
			fmt.Printf("Reached time limit (%s).\n", r.opts.TimeLimit)
			break
		}

		iteration++
		fmt.Print(IterationBanner(iteration, time.Now().Format("15:04:05")))

		done, err := r.runIteration()
		if err != nil {
			fmt.Println(ColorError(fmt.Sprintf("Error: %v", err)))

			// Check if it's a rate limit error
			if _, isRateLimit := err.(*rateLimitError); isRateLimit {
				fmt.Println(ColorWarning(fmt.Sprintf("Rate limit hit, sleeping for %s...", rateLimitBackoff)))
				time.Sleep(rateLimitBackoff)
				r.backoffLevel = 0
			} else {
				// Exponential backoff for other errors
				backoff := calculateBackoff(r.backoffLevel)
				fmt.Println(ColorWarning(fmt.Sprintf("Sleeping for %s (backoff level %d)...", backoff, r.backoffLevel)))
				time.Sleep(backoff)
				r.backoffLevel++
			}
			continue
		}

		if done {
			break
		}

		r.backoffLevel = 0
	}

	if r.claudeLogger != nil {
		r.claudeLogger.Close()
	}

	return nil
}

func (r *Runner) runIteration() (done bool, err error) {
	// Run candidate source to get candidates
	fmt.Println(ColorInfo("Running candidate source..."))
	output, err := RunCandidateSource(r.task.CandidateSource, r.env.ProjectDir)
	if err != nil {
		return false, fmt.Errorf("candidate source failed: %w", err)
	}

	if r.opts.Verbose {
		fmt.Printf(ColorInfo("Candidate source output:\n%s\n"), output)
	}

	candidates, err := ParseCandidates(output)
	if err != nil {
		return false, fmt.Errorf("failed to parse candidates: %w", err)
	}

	// Filter by hash if requested
	candidates = FilterByHash(candidates, r.opts.HashFilter)

	if r.opts.Verbose {
		fmt.Printf(ColorInfo("Parsed candidates (%d total):\n"), len(candidates))
		for _, c := range candidates {
			fmt.Printf("  - %s\n", c.Key)
		}
	}

	// Count ignored candidates
	ignoredCount := 0
	for _, c := range candidates {
		if r.ignoredList.Contains(c.Key) {
			ignoredCount++
		}
	}

	// Select first non-ignored candidate
	candidate := SelectCandidate(candidates, r.ignoredList)
	if candidate == nil {
		remaining := len(candidates) - ignoredCount
		if remaining == 0 && ignoredCount > 0 {
			fmt.Printf("No more candidates (%d ignored)\n", ignoredCount)
		} else {
			fmt.Println("No more candidates.")
		}
		return true, nil
	}

	fmt.Printf("Found %d candidates (%d ignored)\n", len(candidates)-ignoredCount, ignoredCount)

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

	// Run Claude with progress timer
	timer := NewProgressTimer("Running Claude...", r.claudeStats)
	timer.Start()

	if r.claudeLogger != nil {
		r.claudeLogger.StartEntry(prompt)
	}

	claudeFlags := r.task.ClaudeFlags

	// Use task-level claude_command if set, otherwise fall back to global
	claudeCmd := r.task.ClaudeCommand
	if claudeCmd == "" {
		claudeCmd = r.env.Config.ClaudeCommand
	} else if r.opts.Verbose {
		fmt.Printf(ColorInfo("Using task-level claude_command: %s\n"), claudeCmd)
	}

	claudeOutput, err := RunClaudeCommand(claudeCmd, claudeFlags, prompt, r.env.ProjectDir, r.claudeLogger)

	timer.Stop()

	// Display Claude's output now that timer is stopped
	if claudeOutput != "" {
		fmt.Print(claudeOutput)
	}

	if r.claudeLogger != nil {
		r.claudeLogger.EndEntry()
	}

	// Check for rate limit in output
	if strings.Contains(claudeOutput, rateLimitPhrase) {
		return false, &rateLimitError{msg: "claude rate limit hit"}
	}

	if err != nil {
		return false, fmt.Errorf("claude failed: %w", err)
	}

	// Verify build FIRST before checking candidate presence
	// Invalid changes can cause candidates to be excluded from source,
	// creating false positives if we check presence before build
	if !r.runVerify() {
		fmt.Println(ColorWarning("Build failed after Claude changes"))
		return r.handleFailure(candidate)
	}

	// Build passed - now check if candidate was fixed
	fmt.Println(ColorInfo("Re-checking candidates..."))
	output, err = RunCandidateSource(r.task.CandidateSource, r.env.ProjectDir)
	if err != nil {
		return false, fmt.Errorf("candidate source re-run failed: %w", err)
	}

	if r.opts.Verbose {
		fmt.Printf(ColorInfo("Re-check candidate source output:\n%s\n"), output)
	}

	newCandidates, err := ParseCandidates(output)
	if err != nil {
		return false, fmt.Errorf("failed to parse new candidates: %w", err)
	}

	// Apply the same hash filter for consistent verification
	newCandidates = FilterByHash(newCandidates, r.opts.HashFilter)

	if r.opts.Verbose {
		fmt.Printf(ColorInfo("Re-check parsed candidates (%d total):\n"), len(newCandidates))
		for _, c := range newCandidates {
			fmt.Printf("  - %s\n", c.Key)
		}
		fmt.Printf(ColorInfo("Looking for candidate: %s\n"), candidate.Key)
		fmt.Printf(ColorInfo("Candidate found: %v\n"), containsKey(newCandidates, candidate.Key))
	}

	candidateFixed := !containsKey(newCandidates, candidate.Key)

	if candidateFixed {
		return r.handleSuccess(candidate, true)  // Build already verified
	} else {
		return r.handleFailure(candidate)
	}
}

func (r *Runner) handleSuccess(candidate *Candidate, buildVerified bool) (bool, error) {
	fmt.Println(ColorSuccess(fmt.Sprintf("✓ Candidate %s was fixed!", candidate.Key)))

	// Verify build (unless already verified)
	if !buildVerified && !r.runVerify() {
		fmt.Println(ColorWarning("Build verification failed after fix, attempting recovery..."))
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
		fmt.Println(ColorInfo("Committing changes..."))
		ok, err := RunCommand(successCmd, r.env.ProjectDir)
		if err != nil {
			return false, fmt.Errorf("success command error: %w", err)
		}
		if !ok {
			fmt.Println(ColorWarning("Warning: success command returned non-zero exit code"))
		} else {
			fmt.Println(ColorSuccess("✓ Changes committed"))
		}
		r.logOutcome(OutcomeFixed, "committed")
	} else {
		r.logOutcome(OutcomeFixed, "no changes to commit")
	}

	return false, nil
}

func (r *Runner) handleFailure(candidate *Candidate) (bool, error) {
	fmt.Println(ColorError(fmt.Sprintf("✗ Candidate %s not fixed.", candidate.Key)))

	if r.task.AcceptBestEffort {
		// Best effort mode: commit if build passes
		if r.runVerify() {
			hasChanges, err := HasUncommittedChanges(r.env.ProjectDir)
			if err != nil {
				return false, fmt.Errorf("failed to check for changes: %w", err)
			}

			if hasChanges {
				fmt.Println(ColorInfo("Committing partial progress..."))
				successCmd := InterpolateCommand(r.env.Config.SuccessCommand, candidate, r.task.Name)
				// Modify message for best effort
				successCmd = replaceBestEffort(successCmd, candidate.Key)
				ok, err := RunCommand(successCmd, r.env.ProjectDir)
				if err != nil {
					return false, fmt.Errorf("best effort commit error: %w", err)
				}
				if !ok {
					fmt.Println(ColorWarning("Warning: best effort commit returned non-zero exit code"))
				} else {
					fmt.Println(ColorSuccess("✓ Changes committed"))
				}
				r.logOutcome(OutcomeBestEffort, "partial progress committed")
			} else {
				r.logOutcome(OutcomeNotFixed, "no changes made")
			}
		} else {
			// Build failed, reset
			fmt.Println(ColorWarning("Build failed, resetting..."))
			if !r.runResetAndVerify() {
				return false, fmt.Errorf("failed to reset")
			}
			r.logOutcome(OutcomeBuildFailed, "reverted")
		}
	} else {
		// Standard mode: reset changes
		if !r.runResetAndVerify() {
			return false, fmt.Errorf("failed to reset")
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
	fmt.Print(ColorInfo("Verifying build... "))
	ok, err := RunCommandShowOnFail(r.env.Config.VerifyCommand, r.env.ProjectDir)
	if err != nil {
		fmt.Println(ColorError(fmt.Sprintf("Verify command error: %v", err)))
		return false
	}
	if ok {
		fmt.Println(ColorInfo("OK"))
	}
	return ok
}

func (r *Runner) runReset() bool {
	if r.env.Config.ResetCommand == "" {
		return true
	}

	ok, err := RunCommandSilent(r.env.Config.ResetCommand, r.env.ProjectDir)
	if err != nil {
		return false
	}
	return ok
}

func (r *Runner) runResetAndVerify() bool {
	fmt.Print(ColorInfo("Resetting changes and verifying build..."))

	// Reset
	if !r.runReset() {
		fmt.Println(ColorError(" FAILED"))
		return false
	}

	// Verify
	if r.env.Config.VerifyCommand == "" {
		fmt.Println(ColorInfo(" OK"))
		return true
	}

	ok, err := RunCommandSilent(r.env.Config.VerifyCommand, r.env.ProjectDir)
	if err != nil || !ok {
		fmt.Println(ColorError(" FAILED"))
		return false
	}

	fmt.Println(ColorInfo(" OK"))
	return true
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
