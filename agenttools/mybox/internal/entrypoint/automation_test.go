package entrypoint

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	automationinfra "github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/automation"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

type automationTestDispatcher struct{}

func (*automationTestDispatcher) Start(_ context.Context, _ *domain.Task, _, _ string) (string, error) {
	return "test-agent", nil
}

func TestRunAutomationOnceRunsManualOnlyWithID(t *testing.T) {
	root := t.TempDir()
	triggerDir := filepath.Join(root, "_task_triggers", "manual_report")
	require.NoError(t, os.MkdirAll(triggerDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "trigger.yaml"), []byte(`
version: 1
enabled: true
trigger:
  type: manual
task:
  agent_kind: opencode
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "task.md"), []byte("---\ntitle: Manual report\n---\n"), 0o644))

	tasks := application.NewTaskUseCase(
		markdown.NewTaskRepository(root),
		markdown.NewTemplateRenderer(root, ""),
		markdown.NewPromptRepository(root, ""),
		"demo",
		root,
	)
	runs := automationinfra.NewRunStore(filepath.Join(root, "state"))
	ac := &automationContext{
		definitions: automationinfra.NewDefinitionRepository(root),
		useCase: application.NewAutomationUseCase(
			"demo", root, runs, tasks, &automationTestDispatcher{},
		),
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	var output bytes.Buffer
	cmd.SetOut(&output)
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)

	require.NoError(t, runAutomationOnce(cmd, ac, "", now))
	entries, err := os.ReadDir(filepath.Join(root, "_tasks"))
	if os.IsNotExist(err) {
		entries = nil
		err = nil
	}
	require.NoError(t, err)
	assert.Empty(t, entries)

	require.NoError(t, runAutomationOnce(cmd, ac, "manual_report", now))
	entries, err = os.ReadDir(filepath.Join(root, "_tasks"))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Contains(t, output.String(), "dispatched")
}
