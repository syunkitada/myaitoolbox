package application

import (
	"context"
	"io"
	"sort"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type FileUseCase struct {
	Files domain.FileRepository
}

func NewFileUseCase(files domain.FileRepository) *FileUseCase {
	return &FileUseCase{Files: files}
}

func (u *FileUseCase) Tree(ctx context.Context, showHidden bool) ([]domain.FileEntry, error) {
	entries, err := u.Files.Tree(ctx, showHidden)
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

func (u *FileUseCase) Children(ctx context.Context, parent string, showHidden bool) ([]domain.FileEntry, error) {
	if parent != "" {
		if err := validatePath(parent); err != nil {
			return nil, err
		}
	}
	return u.Files.Children(ctx, parent, showHidden)
}

func (u *FileUseCase) MarkdownTags(ctx context.Context) ([]string, error) {
	return u.Files.MarkdownTags(ctx)
}

func (u *FileUseCase) Content(ctx context.Context, path string) (string, error) {
	if err := validatePath(path); err != nil {
		return "", err
	}
	return u.Files.Content(ctx, path)
}

func (u *FileUseCase) Raw(ctx context.Context, path string) ([]byte, error) {
	if err := validatePath(path); err != nil {
		return nil, err
	}
	return u.Files.Raw(ctx, path)
}

func (u *FileUseCase) Save(ctx context.Context, path string, content string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.Save(ctx, path, content)
}

func (u *FileUseCase) SaveBytes(ctx context.Context, path string, content []byte) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.SaveBytes(ctx, path, content)
}

func (u *FileUseCase) SaveReader(ctx context.Context, path string, content io.Reader) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.SaveReader(ctx, path, content)
}

func (u *FileUseCase) Create(ctx context.Context, path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.Create(ctx, path)
}

func (u *FileUseCase) CreateDir(ctx context.Context, path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.CreateDir(ctx, path)
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

func (u *FileUseCase) Execute(ctx context.Context, path string) (domain.FileExecResult, error) {
	if err := validatePath(path); err != nil {
		return domain.FileExecResult{}, err
	}
	return u.Files.Execute(ctx, path)
}
