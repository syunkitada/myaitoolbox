package application

import (
	"context"
	"sort"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type FileUseCase struct {
	Files domain.FileRepository
}

func NewFileUseCase(files domain.FileRepository) *FileUseCase {
	return &FileUseCase{Files: files}
}

func (u *FileUseCase) Tree(ctx context.Context) ([]domain.FileEntry, error) {
	entries, err := u.Files.Tree(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind == domain.FileKindDir
		}
		return entries[i].Path < entries[j].Path
	})
	return entries, nil
}

func (u *FileUseCase) Content(ctx context.Context, path string) (string, error) {
	if err := validatePath(path); err != nil {
		return "", err
	}
	return u.Files.Content(ctx, path)
}
