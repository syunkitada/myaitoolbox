package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/automation"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

type fakeAgentDispatcher struct {
	kind   string
	prompt string
	taskID string
}

func (f *fakeAgentDispatcher) Start(_ context.Context, task *domain.Task, kind, prompt string) (string, error) {
	f.taskID = task.ID
	f.kind = kind
	f.prompt = prompt
	return "agent-1", nil
}

func newAutomationUC(t *testing.T, dispatcher domain.AgentDispatcher) (*AutomationUseCase, *automation.RunStore, string) {
	t.Helper()
	root := t.TempDir()
	triggerDir := filepath.Join(root, "_task_triggers", "daily_report")
	require.NoError(t, os.MkdirAll(triggerDir, 0o755))
	templatePath := filepath.Join(triggerDir, "task.md")
	require.NoError(t, os.WriteFile(templatePath, []byte("---\ntitle: Daily report\npriority: high\n---\n\nPrepare the report."), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "prompts"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "prompts", "run-report.md"), []byte("run `$task_file_path` for $trigger_id"), 0o644))
	store := automation.NewRunStore(filepath.Join(root, "state"))
	tasks := NewTaskUseCase(
		markdown.NewTaskRepository(root),
		markdown.NewTemplateRenderer(root, ""),
		markdown.NewPromptRepository(root, ""),
		"demo",
		root,
	)
	return NewAutomationUseCase("demo", root, store, tasks, dispatcher), store, templatePath
}

func TestAutomationRunEventCreatesAndDispatchesTask(t *testing.T) {
	dispatcher := &fakeAgentDispatcher{}
	uc, store, templatePath := newAutomationUC(t, dispatcher)
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.FixedZone("JST", 9*60*60))
	uc.Now = func() time.Time { return now }
	def := domain.TriggerDefinition{
		ID:           "daily_report",
		Enabled:      true,
		Type:         domain.TriggerTypeCron,
		TemplatePath: templatePath,
		AgentKind:    "opencode",
		Prompt:       "run-report",
		Overlap:      domain.OverlapSkip,
	}
	event := domain.TriggerEvent{ID: "daily_report@2026-09-27T09:00:00+09:00", TriggerID: def.ID, ScheduledAt: now}

	run, err := uc.RunEvent(context.Background(), def, event)
	require.NoError(t, err)
	assert.Equal(t, domain.RunDispatched, run.Status)
	assert.NotEmpty(t, run.TaskID)
	assert.Equal(t, "agent-1", run.AgentName)
	assert.Equal(t, run.TaskID, dispatcher.taskID)
	assert.Equal(t, "opencode", dispatcher.kind)
	assert.Contains(t, dispatcher.prompt, "_tasks/")

	task, err := uc.Tasks.Show(context.Background(), run.TaskID)
	require.NoError(t, err)
	assert.Equal(t, domain.TaskStatusTodo, task.Status)
	assert.Equal(t, domain.TaskPriorityHigh, task.Priority)
	assert.Contains(t, task.Body, "Prepare the report.")

	stored, err := store.Find(context.Background(), "demo", def.ID, event.ID)
	require.NoError(t, err)
	assert.Equal(t, run.TaskID, stored.TaskID)
}

func TestAutomationRunEventIsIdempotent(t *testing.T) {
	dispatcher := &fakeAgentDispatcher{}
	uc, _, templatePath := newAutomationUC(t, dispatcher)
	def := domain.TriggerDefinition{ID: "daily_report", Enabled: true, Type: domain.TriggerTypeCron, TemplatePath: templatePath, AgentKind: "opencode", Overlap: domain.OverlapAllow}
	event := domain.TriggerEvent{ID: "same-event", TriggerID: def.ID, OccurredAt: time.Now()}

	first, err := uc.RunEvent(context.Background(), def, event)
	require.NoError(t, err)
	second, err := uc.RunEvent(context.Background(), def, event)
	require.NoError(t, err)
	assert.Equal(t, first.ID, second.ID)
	assert.Equal(t, first.TaskID, second.TaskID)
	assert.Equal(t, first.Status, second.Status)
}

func TestAutomationRunManualCreatesIndependentRuns(t *testing.T) {
	dispatcher := &fakeAgentDispatcher{}
	uc, store, templatePath := newAutomationUC(t, dispatcher)
	now := time.Date(2026, 9, 27, 9, 0, 0, 0, time.UTC)
	uc.Now = func() time.Time { return now }
	def := domain.TriggerDefinition{
		ID:           "manual_report",
		Enabled:      true,
		Type:         domain.TriggerTypeManual,
		TemplatePath: templatePath,
		AgentKind:    "opencode",
		Overlap:      domain.OverlapSkip,
	}

	first, err := uc.RunManual(context.Background(), def)
	require.NoError(t, err)
	second, err := uc.RunManual(context.Background(), def)
	require.NoError(t, err)

	assert.Equal(t, domain.RunDispatched, first.Status)
	assert.Equal(t, domain.RunDispatched, second.Status)
	assert.NotEqual(t, first.EventID, second.EventID)
	assert.NotEqual(t, first.TaskID, second.TaskID)
	runs, err := store.List(context.Background(), "demo", def.ID)
	require.NoError(t, err)
	assert.Len(t, runs, 2)
}

func TestAutomationRunEventCopiesFileInputIntoTask(t *testing.T) {
	dispatcher := &fakeAgentDispatcher{}
	uc, _, templatePath := newAutomationUC(t, dispatcher)
	inputPath := filepath.Join(uc.ProjectPath, "incoming", "report.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(inputPath), 0o755))
	require.NoError(t, os.WriteFile(inputPath, []byte("source report"), 0o644))
	def := domain.TriggerDefinition{
		ID:           "watch_reports",
		ProjectRoot:  uc.ProjectPath,
		Enabled:      true,
		Type:         domain.TriggerTypeFileCreated,
		TemplatePath: templatePath,
		AgentKind:    "opencode",
		Overlap:      domain.OverlapAllow,
	}
	event := domain.TriggerEvent{ID: "file:watch_reports:incoming/report.md:13:1", TriggerID: def.ID, SourcePath: "incoming/report.md"}

	run, err := uc.RunEvent(context.Background(), def, event)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(uc.ProjectPath, "_tasks", run.TaskID, "input", "report.md"))
	require.NoError(t, err)
	assert.Equal(t, "source report", string(data))
	task, err := uc.Tasks.Show(context.Background(), run.TaskID)
	require.NoError(t, err)
	assert.Contains(t, task.Body, "_tasks/"+run.TaskID+"/input/report.md")
}
