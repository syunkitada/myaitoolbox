package entrypoint

import (
	"context"
	"fmt"

	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	automationinfra "github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/automation"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/config"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

type App struct {
	Config       *domain.Config
	Project      *domain.Project
	Projects     *application.ProjectUseCase
	Tasks        *application.TaskUseCase
	TaskTriggers *application.TaskTriggerUseCase
	Automation   *application.AutomationUseCase
	Files        *application.FileUseCase
	State        *application.StateUseCase
}

func NewApp(ctx context.Context, projectName string) (*App, error) {
	store := config.NewStore()
	cfg, err := store.Load(ctx)
	if err != nil {
		return nil, err
	}
	if len(cfg.Projects) == 0 {
		return nil, fmt.Errorf("no projects configured: run `mybox project add <path>`")
	}
	name := projectName
	if name == "" {
		name = cfg.DefaultProject
	}
	var project *domain.Project
	for i := range cfg.Projects {
		if cfg.Projects[i].Name == name {
			project = &cfg.Projects[i]
			break
		}
	}
	if project == nil {
		return nil, fmt.Errorf("project %q not found", name)
	}
	var defaultPath string
	for i := range cfg.Projects {
		if cfg.Projects[i].Name == cfg.DefaultProject {
			defaultPath = cfg.Projects[i].Path
			break
		}
	}
	taskRepository := markdown.NewTaskRepository(project.Path)
	templateRenderer := markdown.NewTemplateRenderer(project.Path, defaultPath)
	app := &App{
		Config:   cfg,
		Project:  project,
		Projects: application.NewProjectUseCase(store),
		Tasks: application.NewTaskUseCase(
			taskRepository,
			templateRenderer,
			markdown.NewPromptRepository(project.Path, defaultPath),
			project.Name,
			project.Path,
		),
		TaskTriggers: application.NewTaskTriggerUseCase(
			automationinfra.NewDefinitionRepository(project.Path),
			templateRenderer,
		),
		Files: application.NewFileUseCase(markdown.NewFileRepository(project.Path)),
		State: application.NewStateUseCase(config.NewStateStore()),
	}
	app.Automation = application.NewAutomationUseCase(
		app.Project.Name,
		app.Project.Path,
		automationinfra.NewRunStore(config.AutomationStateDir()),
		app.Tasks,
		&automationAgentDispatcher{app: app},
	)
	return app, nil
}

func NewProjectApp() *application.ProjectUseCase {
	return application.NewProjectUseCase(config.NewStore())
}
