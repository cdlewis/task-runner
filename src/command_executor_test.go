package main

// MockCommandExecutor is a test double for CommandExecutor.
type MockCommandExecutor struct {
	// Commands to results mapping
	Results map[string]CommandResult
	// Record of calls made
	Calls []CallRecord
	// Mock for HasUncommittedChanges
	HasChangesResult bool
	HasChangesErr    error
}

// CommandResult represents the result of executing a command.
type CommandResult struct {
	Success bool
	Error   error
}

// CallRecord records a single command execution.
type CallRecord struct {
	Command string
	WorkDir string
}

// NewMockCommandExecutor creates a new mock executor.
func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		Results:          make(map[string]CommandResult),
		Calls:            make([]CallRecord, 0),
		HasChangesResult: false, // Default: no changes
		HasChangesErr:    nil,
	}
}

// Run executes a command, recording the call and returning the configured result.
func (m *MockCommandExecutor) Run(command, workDir string) (bool, error) {
	m.Calls = append(m.Calls, CallRecord{Command: command, WorkDir: workDir})
	if result, ok := m.Results[command]; ok {
		return result.Success, result.Error
	}
	// Default: success
	return true, nil
}

// RunSilent executes a command silently, recording the call and returning the configured result.
func (m *MockCommandExecutor) RunSilent(command, workDir string) (bool, error) {
	m.Calls = append(m.Calls, CallRecord{Command: command, WorkDir: workDir})
	if result, ok := m.Results[command]; ok {
		return result.Success, result.Error
	}
	// Default: success
	return true, nil
}

// RunShowOnFail executes a command, recording the call and returning the configured result.
func (m *MockCommandExecutor) RunShowOnFail(command, workDir string) (bool, error) {
	m.Calls = append(m.Calls, CallRecord{Command: command, WorkDir: workDir})
	if result, ok := m.Results[command]; ok {
		return result.Success, result.Error
	}
	// Default: success
	return true, nil
}

// HasUncommittedChanges returns the configured result.
func (m *MockCommandExecutor) HasUncommittedChanges(workDir string) (bool, error) {
	return m.HasChangesResult, m.HasChangesErr
}

// SetResult sets the result for a specific command.
func (m *MockCommandExecutor) SetResult(command string, success bool, err error) {
	m.Results[command] = CommandResult{Success: success, Error: err}
}

// SetHasChanges sets the result for HasUncommittedChanges.
func (m *MockCommandExecutor) SetHasChanges(hasChanges bool, err error) {
	m.HasChangesResult = hasChanges
	m.HasChangesErr = err
}

// ClearCalls clears the recorded calls.
func (m *MockCommandExecutor) ClearCalls() {
	m.Calls = make([]CallRecord, 0)
}

// CallCount returns the number of times a command was called.
func (m *MockCommandExecutor) CallCount(command string) int {
	count := 0
	for _, call := range m.Calls {
		if call.Command == command {
			count++
		}
	}
	return count
}

// CalledWith returns true if a command was called with the exact arguments.
func (m *MockCommandExecutor) CalledWith(command string) bool {
	for _, call := range m.Calls {
		if call.Command == command {
			return true
		}
	}
	return false
}
