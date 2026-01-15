package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid config",
			yaml: `
claude_command: "/path/to/claude"
verify_command: "cargo check"
success_command: "git commit -m 'Fix: $CANDIDATE'"
reset_command: "git reset --hard"
`,
			wantErr: false,
		},
		{
			name: "unknown field",
			yaml: `
claude_command: "claude"
rofnkjsnfke3: "bad"
`,
			wantErr: true,
		},
		{
			name: "typo in claude_command",
			yaml: `
cluade_command: "claude"
`,
			wantErr: true,
		},
		{
			name: "typo in verify_command",
			yaml: `
claude_command: "claude"
verify_comand: "cargo check"
`,
			wantErr: true,
		},
		{
			name: "minimal valid config",
			yaml: `
claude_command: "claude"
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := loadConfig(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadTaskRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "valid task with prompt",
			yaml: `
candidate_source: "cargo check 2>&1 | grep error"
prompt: "Fix this issue: $INPUT"
`,
			wantErr: false,
		},
		{
			name: "valid task with template",
			yaml: `
candidate_source: "cargo check 2>&1 | grep error"
template: "template.txt"
`,
			wantErr: false,
		},
		{
			name: "valid task with all fields",
			yaml: `
candidate_source: "cargo check 2>&1 | grep error"
prompt: "Fix this issue: $INPUT"
claude_flags: "--fast"
claude_command: "/custom/claude"
accept_best_effort: true
`,
			wantErr: false,
		},
		{
			name: "unknown field",
			yaml: `
candidate_source: "cargo check"
prompt: "fix it"
unknown_field: "value"
`,
			wantErr: true,
		},
		{
			name: "typo in candidate_source",
			yaml: `
candiate_source: "cargo check"
prompt: "fix it"
`,
			wantErr: true,
		},
		{
			name: "typo in accept_best_effort",
			yaml: `
candidate_source: "cargo check"
prompt: "fix it"
accept_best_effort: truee
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpDir := t.TempDir()
			taskPath := filepath.Join(tmpDir, "task.yaml")
			if err := os.WriteFile(taskPath, []byte(tt.yaml), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := loadTask(taskPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadTask() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
