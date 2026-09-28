package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type TaskTriggerInput struct {
	ID   string
	Name string
	// Content nil uses the rendered template; a non-nil value can explicitly
	// provide an empty trigger task.md.
	Content   *string
	Type      domain.TriggerType
	Cron      string
	Timezone  string
	WatchPath string
	Pattern   string
	AgentKind string
	Prompt    string
}

const DefaultTaskTriggerPrompt = "'$task_file_path' を実施してください。"

type TaskTriggerUseCase struct {
	Triggers domain.TriggerRepository
	Template domain.TemplateRenderer
}

func NewTaskTriggerUseCase(triggers domain.TriggerRepository, template domain.TemplateRenderer) *TaskTriggerUseCase {
	return &TaskTriggerUseCase{Triggers: triggers, Template: template}
}

func (u *TaskTriggerUseCase) Find(ctx context.Context, id string) (*domain.TriggerDefinition, error) {
	return u.Triggers.Find(ctx, strings.TrimSpace(id))
}

func (u *TaskTriggerUseCase) Create(ctx context.Context, input TaskTriggerInput) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return fmt.Errorf("%w: task name is required", domain.ErrInvalidArgument)
	}
	agentKind := strings.TrimSpace(input.AgentKind)
	if agentKind == "" {
		return fmt.Errorf("%w: task agent kind is required", domain.ErrInvalidArgument)
	}
	content := ""
	if input.Content != nil {
		content = *input.Content
	} else {
		var err error
		content, err = u.Template.RenderTask(domain.TaskTemplateData{Name: name})
		if err != nil {
			return err
		}
	}
	prompt := strings.TrimSpace(input.Prompt)
	if prompt == "" {
		prompt = DefaultTaskTriggerPrompt
	}
	return u.Triggers.Create(ctx, domain.TriggerDefinitionInput{
		ID:        strings.TrimSpace(input.ID),
		Enabled:   true,
		Type:      input.Type,
		Cron:      strings.TrimSpace(input.Cron),
		Timezone:  strings.TrimSpace(input.Timezone),
		WatchPath: strings.TrimSpace(input.WatchPath),
		Pattern:   strings.TrimSpace(input.Pattern),
		AgentKind: agentKind,
		Prompt:    prompt,
	}, content)
}
