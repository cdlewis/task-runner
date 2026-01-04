package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ClaudeCommand  string `yaml:"claude_command"`
	SuccessCommand string `yaml:"success_command"`
	ResetCommand   string `yaml:"reset_command"`
	VerifyCommand  string `yaml:"verify_command"`
}

type Task struct {
	Name             string // derived from directory name
	Dir              string // path to task directory
	CandidateSource  string `yaml:"candidate_source"`
	Prompt           string `yaml:"prompt"`
	Template         string `yaml:"template"`
	ClaudeFlags      string `yaml:"claude_flags"`
	AcceptBestEffort bool   `yaml:"accept_best_effort"`
}

type Environment struct {
	Config     Config
	Tasks      map[string]Task
	ProjectDir string
	RunnerDir  string
}

func DiscoverEnvironment() (*Environment, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get working directory: %w", err)
	}

	// Look for nigel/ directory first, fall back to task-runner/ for backwards compatibility
	runnerDir := filepath.Join(cwd, "nigel")
	if _, err := os.Stat(runnerDir); os.IsNotExist(err) {
		runnerDir = filepath.Join(cwd, "task-runner")
		if _, err := os.Stat(runnerDir); os.IsNotExist(err) {
			return nil, fmt.Errorf("no nigel/ or task-runner/ directory found in current directory")
		}
	}

	configPath := filepath.Join(runnerDir, "config.yaml")
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Apply defaults
	if config.ClaudeCommand == "" {
		config.ClaudeCommand = "claude"
	}

	// Expand tilde in claude command
	config.ClaudeCommand = expandTilde(config.ClaudeCommand)

	tasks, err := loadTasks(runnerDir)
	if err != nil {
		return nil, fmt.Errorf("failed to load tasks: %w", err)
	}

	return &Environment{
		Config:     *config,
		Tasks:      tasks,
		ProjectDir: cwd,
		RunnerDir:  runnerDir,
	}, nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &config, nil
}

// loadTasks scans runnerDir for subdirectories containing task.yaml files.
func loadTasks(runnerDir string) (map[string]Task, error) {
	tasks := make(map[string]Task)

	entries, err := os.ReadDir(runnerDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read config directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		taskDir := filepath.Join(runnerDir, entry.Name())
		taskFile := filepath.Join(taskDir, "task.yaml")

		if _, err := os.Stat(taskFile); os.IsNotExist(err) {
			continue // not a task directory
		}

		task, err := loadTask(taskFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load task %s: %w", entry.Name(), err)
		}

		task.Name = entry.Name()
		task.Dir = taskDir

		if task.CandidateSource == "" {
			return nil, fmt.Errorf("task %s missing required field 'candidate_source'", entry.Name())
		}
		if task.Prompt == "" && task.Template == "" {
			return nil, fmt.Errorf("task %s must have either 'prompt' or 'template'", entry.Name())
		}
		if task.Prompt != "" && task.Template != "" {
			return nil, fmt.Errorf("task %s cannot have both 'prompt' and 'template'", entry.Name())
		}

		tasks[task.Name] = *task
	}

	return tasks, nil
}

func loadTask(path string) (*Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var task Task
	if err := yaml.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to parse task: %w", err)
	}

	return &task, nil
}

// expandTilde expands ~ to the user's home directory.
func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
