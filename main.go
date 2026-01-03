package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
)

func main() {
	// Define flags
	listFlag := flag.Bool("list", false, "List available tasks")
	limitFlag := flag.Int("limit", 0, "Maximum number of iterations (0 = unlimited)")
	dryRunFlag := flag.Bool("dry-run", false, "Print prompt without executing Claude")
	verboseFlag := flag.Bool("verbose", false, "Print verbose output")
	evensFlag := flag.Bool("evens", false, "Only process candidates with even MD5 hash")
	oddsFlag := flag.Bool("odds", false, "Only process candidates with odd MD5 hash")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: task-runner <task> [options]\n")
		fmt.Fprintf(os.Stderr, "       task-runner --list\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}

	// Reorder args so flags can appear after positional args
	args := reorderArgs(os.Args[1:])
	flag.CommandLine.Parse(args)

	// Validate mutually exclusive flags
	if *evensFlag && *oddsFlag {
		fmt.Fprintln(os.Stderr, ColorError("Error: --evens and --odds are mutually exclusive"))
		os.Exit(1)
	}

	// Discover environment
	env, err := DiscoverEnvironment()
	if err != nil {
		fmt.Fprintln(os.Stderr, ColorError(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}

	// Handle --list
	if *listFlag {
		listTasks(env)
		return
	}

	// Get task name from positional args
	remaining := flag.Args()
	if len(remaining) == 0 {
		fmt.Fprintln(os.Stderr, ColorError("Error: task name required"))
		fmt.Fprintln(os.Stderr, "Use --list to see available tasks")
		os.Exit(1)
	}

	taskName := remaining[0]

	// Determine hash filter
	var hashFilter HashFilter
	if *evensFlag {
		hashFilter = HashFilterEvens
	} else if *oddsFlag {
		hashFilter = HashFilterOdds
	}

	// Create and run the runner
	opts := RunnerOptions{
		Limit:      *limitFlag,
		DryRun:     *dryRunFlag,
		Verbose:    *verboseFlag,
		HashFilter: hashFilter,
	}

	runner, err := NewRunner(env, taskName, opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, ColorError(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}

	if err := runner.Run(); err != nil {
		fmt.Fprintln(os.Stderr, ColorError(fmt.Sprintf("Error: %v", err)))
		os.Exit(1)
	}
}

func listTasks(env *Environment) {
	if len(env.Tasks) == 0 {
		fmt.Println("No tasks found.")
		return
	}

	fmt.Println(ColorBold("Available tasks:"))

	// Sort task names for consistent output
	names := make([]string, 0, len(env.Tasks))
	for name := range env.Tasks {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		task := env.Tasks[name]
		mode := "standard"
		if task.AcceptBestEffort {
			mode = "best-effort"
		}
		fmt.Printf("  %s [%s]\n", ColorInfo(fmt.Sprintf("%-30s", name)), mode)
	}
}

// reorderArgs moves flags before positional arguments so Go's flag package can parse them.
func reorderArgs(args []string) []string {
	var flags, positional []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
			// Check if this flag takes a value (like -limit 5)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				// Check if it's a flag that takes a value
				if arg == "-limit" || arg == "--limit" {
					i++
					flags = append(flags, args[i])
				}
			}
		} else {
			positional = append(positional, arg)
		}
		i++
	}

	return append(flags, positional...)
}
