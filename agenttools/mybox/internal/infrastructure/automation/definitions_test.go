package automation

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestDefinitionRepositoryLoadsCronDefinition(t *testing.T) {
	root := t.TempDir()
	triggerDir := filepath.Join(root, "_task_triggers", "daily_report")
	require.NoError(t, os.MkdirAll(triggerDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "trigger.yaml"), []byte(`
version: 1
enabled: true
trigger:
  type: cron
  cron: "0 9 * * 1-5"
  timezone: Asia/Tokyo
task:
  template: task.md
  agent_kind: opencode
  prompt: do-the-task
execution:
  overlap: skip
  misfire: skip
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "task.md"), []byte("---\ntitle: report\n---\n"), 0o644))

	defs, err := NewDefinitionRepository(root).List(context.Background())
	require.NoError(t, err)
	require.Len(t, defs, 1)
	got := defs[0]
	assert.Equal(t, "daily_report", got.ID)
	assert.True(t, got.Enabled)
	assert.Equal(t, domain.TriggerTypeCron, got.Type)
	assert.Equal(t, "0 9 * * 1-5", got.Cron)
	assert.Equal(t, "Asia/Tokyo", got.Timezone)
	assert.Equal(t, filepath.Join(triggerDir, "task.md"), got.TemplatePath)
	assert.Equal(t, "opencode", got.AgentKind)
	assert.Equal(t, "do-the-task", got.Prompt)
	assert.Equal(t, domain.OverlapSkip, got.Overlap)
	assert.Equal(t, domain.MisfireSkip, got.Misfire)
}

func TestDefinitionRepositoryLoadsFileDefinitionDefaults(t *testing.T) {
	root := t.TempDir()
	triggerDir := filepath.Join(root, "_task_triggers", "watch_notes")
	require.NoError(t, os.MkdirAll(filepath.Join(triggerDir, "incoming"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "trigger.yaml"), []byte(`
version: 1
enabled: true
trigger:
  type: file_created
  path: incoming
task:
  agent_kind: codex
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "task.md"), []byte("---\ntitle: note\n---\n"), 0o644))

	defs, err := NewDefinitionRepository(root).List(context.Background())
	require.NoError(t, err)
	require.Len(t, defs, 1)
	got := defs[0]
	assert.Equal(t, domain.TriggerTypeFileCreated, got.Type)
	assert.Equal(t, filepath.Join(triggerDir, "incoming"), got.WatchPath)
	assert.Equal(t, "*", got.Pattern)
	assert.Equal(t, 2*time.Second, got.SettleFor)
	assert.Equal(t, domain.OverlapAllow, got.Overlap)
}

func TestDefinitionRepositoryLoadsManualDefinition(t *testing.T) {
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
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "task.md"), []byte("---\ntitle: report\n---\n"), 0o644))

	defs, err := NewDefinitionRepository(root).List(context.Background())
	require.NoError(t, err)
	require.Len(t, defs, 1)
	got := defs[0]
	assert.Equal(t, domain.TriggerTypeManual, got.Type)
	assert.Empty(t, got.Cron)
	assert.Empty(t, got.Timezone)
	assert.Empty(t, got.WatchPath)
	assert.Equal(t, domain.OverlapAllow, got.Overlap)
}

func TestDefinitionRepositoryRejectsUnsafeTemplatePath(t *testing.T) {
	root := t.TempDir()
	triggerDir := filepath.Join(root, "_task_triggers", "unsafe")
	require.NoError(t, os.MkdirAll(triggerDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "trigger.yaml"), []byte(`
version: 1
enabled: true
trigger:
  type: cron
  cron: "* * * * *"
  timezone: UTC
task:
  template: ../task.md
  agent_kind: opencode
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(triggerDir, "task.md"), []byte("template"), 0o644))

	_, err := NewDefinitionRepository(root).List(context.Background())
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
}

func TestDefinitionRepositoryCreatesCronDefinition(t *testing.T) {
	root := t.TempDir()
	repo := NewDefinitionRepository(root)

	err := repo.Create(context.Background(), domain.TriggerDefinitionInput{
		ID:        "daily_report",
		Enabled:   true,
		Type:      domain.TriggerTypeCron,
		Cron:      "0 9 * * 1-5",
		Timezone:  "Asia/Tokyo",
		AgentKind: "opencode",
		Prompt:    "do-the-task",
	}, "---\ntitle: Daily report\n---\n\n## TODO\n")
	require.NoError(t, err)

	triggerDir := filepath.Join(root, "_task_triggers", "daily_report")
	assert.FileExists(t, filepath.Join(triggerDir, "trigger.yaml"))
	assert.FileExists(t, filepath.Join(triggerDir, "task.md"))

	defs, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, "daily_report", defs[0].ID)
	assert.Equal(t, domain.TriggerTypeCron, defs[0].Type)
	assert.Equal(t, "0 9 * * 1-5", defs[0].Cron)
	assert.Equal(t, "Asia/Tokyo", defs[0].Timezone)
	assert.Equal(t, "opencode", defs[0].AgentKind)
	assert.Equal(t, "do-the-task", defs[0].Prompt)
}

func TestDefinitionRepositoryCreatesManualDefinitionWithoutWatchDirectory(t *testing.T) {
	root := t.TempDir()
	repo := NewDefinitionRepository(root)

	err := repo.Create(context.Background(), domain.TriggerDefinitionInput{
		ID:        "manual_report",
		Enabled:   true,
		Type:      domain.TriggerTypeManual,
		AgentKind: "opencode",
		Prompt:    "do-the-task",
	}, "---\ntitle: Manual report\n---\n")
	require.NoError(t, err)

	triggerDir := filepath.Join(root, "_task_triggers", "manual_report")
	assert.FileExists(t, filepath.Join(triggerDir, "trigger.yaml"))
	assert.FileExists(t, filepath.Join(triggerDir, "task.md"))
	assert.NoDirExists(t, filepath.Join(triggerDir, "incoming"))

	defs, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, domain.TriggerTypeManual, defs[0].Type)
}

func TestDefinitionRepositoryCreatesFileDefinitionAndWatchDirectory(t *testing.T) {
	root := t.TempDir()
	repo := NewDefinitionRepository(root)

	err := repo.Create(context.Background(), domain.TriggerDefinitionInput{
		ID:        "watch_notes",
		Enabled:   true,
		Type:      domain.TriggerTypeFileCreated,
		WatchPath: "incoming/reports",
		Pattern:   "*.md",
		AgentKind: "codex",
	}, "---\ntitle: Notes\n---\n")
	require.NoError(t, err)

	watchPath := filepath.Join(root, "_task_triggers", "watch_notes", "incoming", "reports")
	info, err := os.Stat(watchPath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	defs, err := repo.List(context.Background())
	require.NoError(t, err)
	require.Len(t, defs, 1)
	assert.Equal(t, watchPath, defs[0].WatchPath)
	assert.Equal(t, "*.md", defs[0].Pattern)
}

func TestDefinitionRepositoryDoesNotOverwriteExistingDefinition(t *testing.T) {
	root := t.TempDir()
	repo := NewDefinitionRepository(root)
	input := domain.TriggerDefinitionInput{
		ID:        "same",
		Enabled:   true,
		Type:      domain.TriggerTypeCron,
		Cron:      "0 9 * * *",
		Timezone:  "UTC",
		AgentKind: "opencode",
	}
	require.NoError(t, repo.Create(context.Background(), input, "first"))

	err := repo.Create(context.Background(), input, "second")
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
	content, readErr := os.ReadFile(filepath.Join(root, "_task_triggers", "same", "task.md"))
	require.NoError(t, readErr)
	assert.Equal(t, "first", string(content))
}
