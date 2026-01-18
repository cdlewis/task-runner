# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Build the binary
go build -o bin/nigel ./src

# Run tests
go test ./src/...

# Run the tool
bin/nigel <task-name>

# List available tasks
bin/nigel --list
```

## Architecture

Nigel is a CLI tool that automates iterative code improvements using Claude AI. It follows a simple loop: identify issues via candidate sources, send them to Claude for fixing, verify results, and commit successful changes.

### Core Components

- **src/main.go** - CLI entry point with flag parsing. Reorders args so flags can appear after positional arguments.
- **src/config.go** - Loads configuration from `nigel/config.yaml` (global settings) and `nigel/<task>/task.yaml` (per-task). Also supports `task-runner/` for backwards compatibility. Contains `Environment` struct that holds all runtime config.
- **src/runner.go** - Main execution loop (`Runner.Run`). Handles iterations, graceful shutdown (SIGQUIT), and consecutive failure backoff (3 failures → 5 min sleep).
- **src/executor.go** - Shell command execution, prompt interpolation, and Claude invocation. Streams Claude output to both stdout and log file.
- **src/candidate.go** - Parses JSON output from candidate sources into candidates. Supports both string and array formats. Manages ignored list (processed candidates) and hash-based filtering for parallel runners.
- **src/logger.go** - Logs Claude interactions to `claude.log` with timestamps.

### Execution Flow

1. `DiscoverEnvironment()` finds `nigel/` directory (or `task-runner/` for backwards compatibility) and loads configs
2. `Runner.Run()` iterates until done or limit reached
3. Each iteration: run candidate source → select candidate → build prompt → invoke Claude → verify fix → commit or reset
4. Processed candidates stored in `ignored.log` to prevent reprocessing (unless `ignore_list` task option is set)

### Task Configuration Options

Tasks can be configured in `nigel/<task>/task.yaml`:

- `candidate_source` - Command that outputs JSON array of candidates
- `prompt` - Inline prompt template (mutually exclusive with `template`)
- `template` - Path to prompt template file (mutually exclusive with `prompt`)
- `claude_flags` - Additional flags to pass to Claude
- `claude_command` - Override Claude command (also available as global config)
- `accept_best_effort` - If true, commit changes even if Claude indicates partial success
- `timeout` - Per-candidate timeout duration
- `ignore_list` - Command that outputs list of already-processed keys (one per line). Use `echo -n` to disable ignoring and reprocess all candidates. If not specified, defaults to reading from `ignored.log` file.
- `repeat` - Retry each candidate up to N times. If a fix works, the candidate disappears from the source output and retries stop naturally. If the fix fails, the candidate persists and gets retried until the attempt count reaches N. Default is 0 (process each candidate once).

### Prompt Variable Interpolation

Prompts support: `$INPUT`, `$INPUT[n]`, `$INPUT[n:]`, `$INPUT["key"]`, `$TASK_ID`
Commands support: `$CANDIDATE`, `$TASK_NAME`

- `$TASK_ID` - A unique random int64 generated per run, useful for tracking or deduplication

## Test Environment

A `test-environment/` directory exists for integration testing:

```bash
cd test-environment
../bin/nigel demo-task
```

The test environment uses `mock-claude`, a bash script that simulates Claude's behavior:

- Accepts `-p` flag for prompts (like real Claude)
- Configurable via `MOCK_CLAUDE_DELAY` (default: 3s) and `MOCK_CLAUDE_FIX` (0/1)
- Creates `.fixed-$CANDIDATE` files when `MOCK_CLAUDE_FIX=1`
- Outputs mock responses for testing iteration flow

To reset state between runs:
```bash
rm nigel/demo-task/*.log .fixed-*
```

### Smoke Testing

**Before committing any changes**, run the smoke test suite to verify basic functionality:

```bash
cd test-environment
./test-smoke.sh
```

The smoke test (`test-smoke.sh`) validates:
- Normal behavior (quick operations, no timers)
- Slow candidate source (delayed progress timer after 5s)
- Slow Claude response (inactivity timer after 30s)

This ensures changes don't break core UI behaviors like progress display and timer management.
