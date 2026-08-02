package application

import (
	"context"
	"path/filepath"
	"sort"
	"time"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type KnowledgeUseCase struct {
	Knowledge domain.KnowledgeRepository
	Template  domain.TemplateRenderer
}

type KnowledgeFilter struct {
	Tag string
}

func NewKnowledgeUseCase(knowledge domain.KnowledgeRepository, template domain.TemplateRenderer) *KnowledgeUseCase {
	return &KnowledgeUseCase{Knowledge: knowledge, Template: template}
}

func (u *KnowledgeUseCase) List(ctx context.Context, filter KnowledgeFilter) ([]domain.Knowledge, error) {
	list, err := u.Knowledge.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []domain.Knowledge
	for _, k := range list {
		if filter.Tag != "" && !contains(k.Tags, filter.Tag) {
			continue
		}
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func (u *KnowledgeUseCase) Show(ctx context.Context, path string) (*domain.Knowledge, error) {
	return u.Knowledge.Find(ctx, path)
}

func (u *KnowledgeUseCase) Content(ctx context.Context, path string) (string, error) {
	return u.Knowledge.RawContent(ctx, path)
}

func (u *KnowledgeUseCase) Create(ctx context.Context, path string) (*domain.Knowledge, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}
	now := time.Now()
	name := filepath.Base(path)
	content, err := u.Template.RenderKnowledge(domain.KnowledgeTemplateData{
		Path:    path,
		Name:    name,
		Created: now,
	})
	if err != nil {
		return nil, err
	}
	if err := u.Knowledge.Create(ctx, path, content); err != nil {
		return nil, err
	}
	return u.Knowledge.Find(ctx, path)
}

func (u *KnowledgeUseCase) SaveContent(ctx context.Context, path string, content string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Knowledge.SaveContent(ctx, path, content)
}

func (u *KnowledgeUseCase) Move(ctx context.Context, oldPath string, newPath string) error {
	if err := validatePath(oldPath); err != nil {
		return err
	}
	if err := validatePath(newPath); err != nil {
		return err
	}
	return u.Knowledge.Move(ctx, oldPath, newPath)
}

func (u *KnowledgeUseCase) Rename(ctx context.Context, oldPath string, newName string) error {
	if err := validatePath(oldPath); err != nil {
		return err
	}
	if newName == "" || filepath.Base(newName) != newName {
		return domain.ErrInvalidPath
	}
	return u.Knowledge.Rename(ctx, oldPath, newName)
}
