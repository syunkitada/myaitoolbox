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

func (u *FileUseCase) Save(ctx context.Context, path string, content string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.Save(ctx, path, content)
}

func (u *FileUseCase) Move(ctx context.Context, oldPath string, newPath string) error {
	if err := validatePath(oldPath); err != nil {
		return err
	}
	if err := validatePath(newPath); err != nil {
		return err
	}
	return u.Files.Move(ctx, oldPath, newPath)
}

func (u *FileUseCase) Copy(ctx context.Context, oldPath string, newPath string) error {
	if err := validatePath(oldPath); err != nil {
		return err
	}
	if err := validatePath(newPath); err != nil {
		return err
	}
	return u.Files.Copy(ctx, oldPath, newPath)
}

func (u *FileUseCase) Delete(ctx context.Context, path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.Delete(ctx, path)
}
