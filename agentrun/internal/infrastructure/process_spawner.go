package infrastructure

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
)

// DefaultAgentTimeout is the maximum time an agent subprocess can run.
const DefaultAgentTimeout = 10 * time.Minute

// ProcessSpawner implements domain.AgentSpawner by executing commands as subprocesses.
type ProcessSpawner struct {
	timeout time.Duration
	workspaceRoot string
}

// NewProcessSpawner creates a ProcessSpawner with the given timeout.
func NewProcessSpawner(workspaceRoot string, timeout time.Duration) *ProcessSpawner {
	if timeout == 0 {
		timeout = DefaultAgentTimeout
	}
	return &ProcessSpawner{
		timeout:       timeout,
		workspaceRoot: workspaceRoot,
	}
}

// SpawnResult contains the output of a subprocess execution.
type SpawnResult struct {
	ExitCode int
	Output   string
	Stderr   string
}

// SpawnCommand runs a command as a subprocess with timeout control.
func (s *ProcessSpawner) SpawnCommand(command string, args []string, workDir string) (*domain.AgentSpawnerResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("failed to start agent process: %w", err)
		}
	}

	return &domain.AgentSpawnerResult{
		ExitCode: exitCode,
		Output:   stdout.String(),
		Stderr:   stderr.String(),
	}, nil
}

// SpawnAgent implements domain.AgentSpawner. It delegates to the LLM adapter
// to build the command, then executes it.
func (s *ProcessSpawner) Spawn(taskInfo *domain.TaskInfo, agentDir string) (*domain.AgentSpawnerResult, error) {
	adapter := NewOpenCodeAdapter(s.workspaceRoot)
	command, args := adapter.BuildCommand(taskInfo)

	// Resolve agent directory relative to workspace
	fullAgentDir := agentDir
	if !filepath.IsAbs(agentDir) {
		fullAgentDir = filepath.Join(s.workspaceRoot, agentDir)
	}

	return s.SpawnCommand(command, args, fullAgentDir)
}
