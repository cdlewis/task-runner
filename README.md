# Task Runner

A Go CLI tool that automates iterative code improvements using Claude AI. It identifies issues via custom candidate sources, sends them to Claude for fixing, verifies the results, and commits successful improvements.

## Installation

```bash
go build -o bin/task-runner
```

Or build to the current directory:

```bash
go build
```

Requires Go 1.21 or later.

## Quick Start

1. Create a `task-runner/` directory in your project root
2. Add a `config.yaml` with global settings
3. Create task directories with `task.yaml` files
4. Run: `task-runner <task-name>`

## Configuration

### Directory Structure

```
project-root/
├── task-runner/
│   ├── config.yaml           # Global configuration
│   └── mytask/               # Task directory
│       ├── task.yaml         # Task definition
│       └── template.txt      # Optional prompt template
```

### config.yaml (Global)

```yaml
claude_command: "claude"                      # Path to Claude CLI
success_command: "git commit -m 'Fix: $CANDIDATE'"  # Run on successful fix
reset_command: "git reset --hard"             # Reset changes on failure
verify_command: "cargo check"                 # Verify build after fix
```

### task.yaml (Per-Task)

```yaml
candidate_source: "cargo check 2>&1 | grep error"    # Command to find candidates
prompt: "Fix this issue: $ARGUMENT"        # Prompt text (or use template)
template: "template.txt"                   # Load prompt from file instead
claude_flags: "--fast"                     # Optional Claude CLI flags
accept_best_effort: false                  # Accept partial fixes
```

## Usage

```bash
# List available tasks
task-runner --list

# Run a task
task-runner mytask

# Run with iteration limit
task-runner mytask --limit 10

# Preview prompts without executing
task-runner mytask --dry-run --verbose

# Distribute work across parallel runners
task-runner mytask --evens   # Process candidates with even MD5 hash
task-runner mytask --odds    # Process candidates with odd MD5 hash
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `--list` | List all available tasks |
| `--limit N` | Maximum iterations (0 = unlimited) |
| `--dry-run` | Print prompts without executing Claude |
| `--verbose` | Print full prompt content |
| `--evens` | Only process candidates with even hash |
| `--odds` | Only process candidates with odd hash |

## How It Works

1. Runs your candidate source to identify **candidates** (issues to fix)
2. Selects the first unprocessed candidate
3. Sends candidate details to Claude with your templated prompt
4. Verifies the fix by re-running the candidate source
5. Commits successful fixes or resets on failure
6. Repeats until done or limit reached

### Task Modes

- **Standard Mode** (`accept_best_effort: false`): Resets changes if fix doesn't fully resolve the candidate
- **Best-Effort Mode** (`accept_best_effort: true`): Commits partial improvements even if candidate isn't fully fixed

## Prompt Templates

Templates support variable interpolation:

| Variable | Description |
|----------|-------------|
| `$ARGUMENT` | First element of candidate |
| `$ARGUMENT_1`, `$ARGUMENT_2`, ... | Specific elements |
| `$REMAINING_ARGUMENTS` | Elements 2+ joined by comma |
| `$CANDIDATE` | Full candidate key (in commands) |

### Candidate Format

Candidates can be simple strings or arrays:
```
simple_issue
["issue_key", "related_info", "more_context"]
```

## Generated Files

Each task directory will contain auto-generated files:

- `ignored.log` - Processed candidates (prevents reprocessing)
- `claude.log` - Full Claude interaction logs for auditing

## Controls

- Press `Ctrl+\` (SIGQUIT) to gracefully stop after the current iteration
- After 3 consecutive failures, the tool sleeps for 5 minutes before retrying

## License

MIT
