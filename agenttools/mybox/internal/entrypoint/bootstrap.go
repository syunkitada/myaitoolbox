package entrypoint

import (
	"context"
	"fmt"

	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/config"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

type App struct {
	Config    *domain.Config
	Project   *domain.Project
	Projects  *application.ProjectUseCase
	Tasks     *application.TaskUseCase
	Knowledge *application.KnowledgeUseCase
	Files     *application.FileUseCase
	Search    *application.SearchUseCase
	State     *application.StateUseCase
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
	app := &App{
		Config:   cfg,
		Project:  project,
		Projects: application.NewProjectUseCase(store),
		Tasks: application.NewTaskUseCase(
			markdown.NewTaskRepository(project.Path),
			markdown.NewTemplateRenderer(project.Path, defaultPath),
			project.Name,
		),
		Knowledge: application.NewKnowledgeUseCase(
			markdown.NewKnowledgeRepository(project.Path),
			markdown.NewTemplateRenderer(project.Path, defaultPath),
		),
		Files:  application.NewFileUseCase(markdown.NewFileRepository(project.Path)),
		Search: application.NewSearchUseCase(markdown.NewSearcher(project.Path)),
		State:  application.NewStateUseCase(config.NewStateStore()),
	}
	return app, nil
}

func NewProjectApp() *application.ProjectUseCase {
	return application.NewProjectUseCase(config.NewStore())
}
