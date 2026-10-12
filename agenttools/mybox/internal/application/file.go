package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type FileUseCase struct {
	Files   domain.FileRepository
	State   *StateUseCase
	Project string
}

const maxFileSearchResults = 100

type limitedFileSearcher interface {
	SearchLimited(ctx context.Context, query string, showHidden bool, limit int) ([]domain.FileSearchResult, int, error)
}

func NewFileUseCase(files domain.FileRepository) *FileUseCase {
	return &FileUseCase{Files: files}
}

// SetState connects file moves to project-scoped favorites and recent files.
// It is optional so repository-only use cases remain independent of persisted
// UI state.
func (u *FileUseCase) SetState(state *StateUseCase, project string) {
	u.State = state
	u.Project = project
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

func (u *FileUseCase) Search(ctx context.Context, query string, showHidden bool) ([]domain.FileSearchResult, int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, 0, fmt.Errorf("%w: search query must not be empty", domain.ErrInvalidArgument)
	}
	if searcher, ok := u.Files.(limitedFileSearcher); ok {
		return searcher.SearchLimited(ctx, query, showHidden, maxFileSearchResults)
	}
	results, err := u.Files.Search(ctx, query, showHidden)
	if err != nil {
		return nil, 0, err
	}
	total := len(results)
	if len(results) > maxFileSearchResults {
		results = results[:maxFileSearchResults]
	}
	return results, total, nil
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
	_, err := u.MoveWithTarget(ctx, oldPath, newPath)
	return err
}

type moveTargetResolver interface {
	MoveTarget(ctx context.Context, oldPath string, newPath string) (string, error)
}

func (u *FileUseCase) MoveWithTarget(ctx context.Context, oldPath string, newPath string) (string, error) {
	if err := validatePath(oldPath); err != nil {
		return "", err
	}
	if err := validatePath(newPath); err != nil {
		return "", err
	}
	effectivePath := newPath
	if resolver, ok := u.Files.(moveTargetResolver); ok {
		var err error
		effectivePath, err = resolver.MoveTarget(ctx, oldPath, newPath)
		if err != nil {
			return "", err
		}
	} else {
		if err := u.Files.Move(ctx, oldPath, newPath); err != nil {
			return "", err
		}
	}
	if u.State == nil {
		return effectivePath, nil
	}
	if err := u.State.MovePath(ctx, oldPath, effectivePath, u.Project); err != nil {
		rollbackErr := u.Files.Move(context.WithoutCancel(ctx), effectivePath, oldPath)
		if rollbackErr != nil {
			return "", errors.Join(err, fmt.Errorf("rollback file move: %w", rollbackErr))
		}
		return "", err
	}
	return effectivePath, nil
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

func (u *FileUseCase) ValidateExecutable(ctx context.Context, path string) error {
	if err := validatePath(path); err != nil {
		return err
	}
	return u.Files.ValidateExecutable(ctx, path)
}

func (u *FileUseCase) Execute(ctx context.Context, path string) (domain.FileExecResult, error) {
	if err := validatePath(path); err != nil {
		return domain.FileExecResult{}, err
	}
	return u.Files.Execute(ctx, path)
}

func (u *FileUseCase) ExecuteStream(ctx context.Context, path string, output io.Writer) (domain.FileExecResult, error) {
	if err := validatePath(path); err != nil {
		return domain.FileExecResult{}, err
	}
	return u.Files.ExecuteStream(ctx, path, output)
}
