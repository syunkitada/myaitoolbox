package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type fakeTriggerRepository struct {
	input   domain.TriggerDefinitionInput
	content string
}

func (f *fakeTriggerRepository) List(context.Context) ([]domain.TriggerDefinition, error) {
	return nil, nil
}

func (f *fakeTriggerRepository) Find(context.Context, string) (*domain.TriggerDefinition, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeTriggerRepository) Create(_ context.Context, input domain.TriggerDefinitionInput, content string) error {
	f.input = input
	f.content = content
	return nil
}

type fakeTaskTemplateRenderer struct {
	content string
}

func (f fakeTaskTemplateRenderer) RenderTask(data domain.TaskTemplateData) (string, error) {
	return f.content + ":" + data.Name, nil
}

func TestTaskTriggerUseCaseRendersTaskAndCreatesDefinition(t *testing.T) {
	repo := &fakeTriggerRepository{}
	uc := NewTaskTriggerUseCase(repo, fakeTaskTemplateRenderer{content: "rendered"})

	err := uc.Create(context.Background(), TaskTriggerInput{
		ID:        "daily_report",
		Name:      "Daily report",
		Type:      domain.TriggerTypeCron,
		Cron:      "0 9 * * 1-5",
		Timezone:  "Asia/Tokyo",
		AgentKind: "opencode",
		Prompt:    "do-the-task",
	})

	require.NoError(t, err)
	assert.Equal(t, domain.TriggerDefinitionInput{
		ID:        "daily_report",
		Enabled:   true,
		Type:      domain.TriggerTypeCron,
		Cron:      "0 9 * * 1-5",
		Timezone:  "Asia/Tokyo",
		AgentKind: "opencode",
		Prompt:    "do-the-task",
	}, repo.input)
	assert.Equal(t, "rendered:Daily report", repo.content)
}

func TestTaskTriggerUseCaseUsesDefaultPrompt(t *testing.T) {
	repo := &fakeTriggerRepository{}
	uc := NewTaskTriggerUseCase(repo, fakeTaskTemplateRenderer{content: "rendered"})

	err := uc.Create(context.Background(), TaskTriggerInput{
		ID:        "daily_report",
		Name:      "Daily report",
		Type:      domain.TriggerTypeCron,
		Cron:      "0 9 * * 1-5",
		Timezone:  "Asia/Tokyo",
		AgentKind: "opencode",
	})

	require.NoError(t, err)
	assert.Equal(t, "'$task_file_path' を実施してください。", repo.input.Prompt)
}

func TestTaskTriggerUseCaseUsesProvidedContent(t *testing.T) {
	repo := &fakeTriggerRepository{}
	uc := NewTaskTriggerUseCase(repo, fakeTaskTemplateRenderer{content: "rendered"})
	content := "---\ntitle: Custom\n---\n\n## User content\n"

	err := uc.Create(context.Background(), TaskTriggerInput{
		ID:        "custom",
		Name:      "Custom task",
		Type:      domain.TriggerTypeManual,
		AgentKind: "codex",
		Content:   &content,
	})

	require.NoError(t, err)
	assert.Equal(t, content, repo.content)
}

func TestTaskTriggerUseCaseRequiresTaskNameAndAgentKind(t *testing.T) {
	uc := NewTaskTriggerUseCase(&fakeTriggerRepository{}, fakeTaskTemplateRenderer{})

	err := uc.Create(context.Background(), TaskTriggerInput{ID: "trigger"})
	assert.ErrorIs(t, err, domain.ErrInvalidArgument)

	err = uc.Create(context.Background(), TaskTriggerInput{ID: "trigger", Name: "Task"})
	assert.ErrorIs(t, err, domain.ErrInvalidArgument)
}
