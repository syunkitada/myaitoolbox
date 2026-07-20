package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/syunkitada/myaitoolbox/agentcrawl/internal/domain"
	"gopkg.in/yaml.v3"
)

// TaskWriter creates task directories with initial files.
type TaskWriter struct {
	workspaceRoot string
}

// NewTaskWriter creates a TaskWriter for the given workspace root.
func NewTaskWriter(workspaceRoot string) *TaskWriter {
	return &TaskWriter{workspaceRoot: workspaceRoot}
}

// taskMetadata is the initial metadata.yaml content.
type taskMetadata struct {
	ID              string `yaml:"id"`
	Title           string `yaml:"title"`
	Status          string `yaml:"status"`
	CurrentAssignee string `yaml:"current_assignee"`
	Priority        string `yaml:"priority"`
	RetryCount      int    `yaml:"retry_count"`
	MaxRetry        int    `yaml:"max_retry"`
	CreatedAt       string `yaml:"created_at"`
	UpdatedAt       string `yaml:"updated_at"`
	Source          string `yaml:"source"`
}

// WriteTask creates the task directory structure and moves the event file into it.
func (w *TaskWriter) WriteTask(taskID string, event *domain.Event, title string) error {
	taskDir := filepath.Join(w.workspaceRoot, "tasks", taskID)

	// Create subdirectories
	for _, sub := range []string{"event", "artifacts", "history"} {
		if err := os.MkdirAll(filepath.Join(taskDir, sub), 0755); err != nil {
			return fmt.Errorf("failed to create %s/: %w", sub, err)
		}
	}

	// Write metadata.yaml
	meta := taskMetadata{
		ID:              taskID,
		Title:           title,
		Status:          "inbox",
		CurrentAssignee: "task-manager*",
		Priority:        "normal",
		RetryCount:      0,
		MaxRetry:        3,
		CreatedAt:       domain.NowISO(),
		UpdatedAt:       domain.NowISO(),
		Source:          "file",
	}
	metaData, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(taskDir, "metadata.yaml"), metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata.yaml: %w", err)
	}

	// Write task.md (template)
	taskMD := fmt.Sprintf("# Task: %s\n\nEvent fileを解析してください。\n\n結果を artifacts/report.md へ保存してください。\n\n必要であれば handoff.yaml を書いてください。（純粋なYAML形式のみ出力し、Markdown装飾を含めないこと）\n", title)
	if err := os.WriteFile(filepath.Join(taskDir, "task.md"), []byte(taskMD), 0644); err != nil {
		return fmt.Errorf("failed to write task.md: %w", err)
	}

	// Move event file into event/ directory
	dst := filepath.Join(taskDir, "event", event.Name)
	if err := os.Rename(event.Path, dst); err != nil {
		return fmt.Errorf("failed to move event file: %w", err)
	}

	return nil
}
