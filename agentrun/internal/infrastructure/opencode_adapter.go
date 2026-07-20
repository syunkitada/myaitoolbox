package infrastructure

import (
	"path/filepath"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
)

// LLMSpawner defines the interface for building LLM subprocess commands.
// Phase 1 implements OpenCode; future phases can add Claude Code, Codex CLI, etc.
type LLMSpawner interface {
	// BuildCommand returns the command and arguments to execute the agent.
	BuildCommand(taskInfo *domain.TaskInfo) (command string, args []string)
}

// OpenCodeAdapter builds the "opencode run" command.
type OpenCodeAdapter struct {
	workspaceRoot string
}

// NewOpenCodeAdapter creates an OpenCodeAdapter.
func NewOpenCodeAdapter(workspaceRoot string) *OpenCodeAdapter {
	return &OpenCodeAdapter{workspaceRoot: workspaceRoot}
}

// BuildCommand constructs the opencode run command for the given task.
// Convention: opencode run --dir agents/<agent-class> --file tasks/<task-id>/task.md --thinking --format json
func (a *OpenCodeAdapter) BuildCommand(taskInfo *domain.TaskInfo) (string, []string) {
	agentClass := extractAgentClass(taskInfo.Metadata.CurrentAssignee)
	agentDir := filepath.Join("agents", agentClass)
	taskFile := filepath.Join("tasks", taskInfo.TaskID, "task.md")

	return "opencode", []string{
		"run",
		"--dir", agentDir,
		"--file", taskFile,
		"--thinking",
		"--format", "json",
	}
}

// extractAgentClass extracts the agent class name from the assignee string.
// e.g., "system-operator*" -> "system-operator"
// e.g., "task-manager1" -> "task-manager"
func extractAgentClass(assignee string) string {
	// Remove trailing wildcard or digits to get the class name
	result := assignee
	for i := len(result) - 1; i >= 0; i-- {
		c := result[i]
		if c == '*' || (c >= '0' && c <= '9') {
			result = result[:i]
		} else {
			break
		}
	}
	return result
}
