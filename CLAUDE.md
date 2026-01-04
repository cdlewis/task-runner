# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build and Run Commands

```bash
# Build the binary
go build

# Run tests (none currently exist)
go test ./...

# Run the tool
./nigel <task-name>

# List available tasks
./nigel --list
```

## Architecture

Nigel is a CLI tool that automates iterative code improvements using Claude AI. It follows a simple loop: identify issues via candidate sources, send them to Claude for fixing, verify results, and commit successful changes.

### Core Components

- **main.go** - CLI entry point with flag parsing. Reorders args so flags can appear after positional arguments.
- **config.go** - Loads configuration from `nigel/config.yaml` (global settings) and `nigel/<task>/task.yaml` (per-task). Also supports `task-runner/` for backwards compatibility. Contains `Environment` struct that holds all runtime config.
- **runner.go** - Main execution loop (`Runner.Run`). Handles iterations, graceful shutdown (SIGQUIT), and consecutive failure backoff (3 failures → 5 min sleep).
- **executor.go** - Shell command execution, prompt interpolation, and Claude invocation. Streams Claude output to both stdout and log file.
- **candidate.go** - Parses JSON output from candidate sources into candidates. Supports both string and array formats. Manages ignored list (processed candidates) and hash-based filtering for parallel runners.
- **logger.go** - Logs Claude interactions to `claude.log` with timestamps.

### Execution Flow

1. `DiscoverEnvironment()` finds `nigel/` directory (or `task-runner/` for backwards compatibility) and loads configs
2. `Runner.Run()` iterates until done or limit reached
3. Each iteration: run candidate source → select candidate → build prompt → invoke Claude → verify fix → commit or reset
4. Processed candidates stored in `ignored.log` to prevent reprocessing

### Prompt Variable Interpolation

Prompts support: `$ARGUMENT`, `$ARGUMENT_1`, `$ARGUMENT_2`, `$REMAINING_ARGUMENTS`
Commands support: `$CANDIDATE`, `$TASK_NAME`
