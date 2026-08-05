package markdown

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type FileRepository struct {
	root string
}

func NewFileRepository(root string) *FileRepository {
	return &FileRepository{root: root}
}

func (r *FileRepository) Tree(ctx context.Context) ([]domain.FileEntry, error) {
	var entries []domain.FileEntry
	if _, err := os.Stat(r.root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	err := filepath.WalkDir(r.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == r.root {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, relErr := filepath.Rel(r.root, path)
		if relErr != nil {
			return relErr
		}
		kind := domain.FileKindFile
		if d.IsDir() {
			kind = domain.FileKindDir
		}
		entries = append(entries, domain.FileEntry{
			Path: filepath.ToSlash(rel),
			Name: d.Name(),
			Kind: kind,
		})
		return nil
	})
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

func (r *FileRepository) Content(ctx context.Context, path string) (string, error) {
	if err := validateFilePath(path); err != nil {
		return "", err
	}
	file := filepath.Join(r.root, filepath.FromSlash(path))
	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return "", err
	}
	if info.IsDir() {
		return "", fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, path)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (r *FileRepository) Save(ctx context.Context, path string, content string) error {
	if err := validateFilePath(path); err != nil {
		return err
	}
	file := filepath.Join(r.root, filepath.FromSlash(path))
	if info, err := os.Stat(file); err == nil && info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, path)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, []byte(content), 0o644)
}

func (r *FileRepository) Move(ctx context.Context, oldPath string, newPath string) error {
	if err := validateFilePath(oldPath); err != nil {
		return err
	}
	if err := validateFilePath(newPath); err != nil {
		return err
	}
	oldFile := filepath.Join(r.root, filepath.FromSlash(oldPath))
	if info, err := os.Stat(oldFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, oldPath)
		}
		return err
	} else if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, oldPath)
	}
	target := filepath.Join(r.root, filepath.FromSlash(newPath))
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		target = filepath.Join(target, filepath.Base(oldFile))
	}
	if target == oldFile {
		return nil
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, newPath)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if isGitRepo(r.root) {
		if err := runGit(ctx, r.root, "mv", oldFile, target); err == nil {
			return nil
		}
	}
	return os.Rename(oldFile, target)
}

func (r *FileRepository) Copy(ctx context.Context, oldPath string, newPath string) error {
	if err := validateFilePath(oldPath); err != nil {
		return err
	}
	if err := validateFilePath(newPath); err != nil {
		return err
	}
	oldFile := filepath.Join(r.root, filepath.FromSlash(oldPath))
	if info, err := os.Stat(oldFile); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, oldPath)
		}
		return err
	} else if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, oldPath)
	}
	target := filepath.Join(r.root, filepath.FromSlash(newPath))
	if info, err := os.Stat(target); err == nil && info.IsDir() {
		target = filepath.Join(target, filepath.Base(oldFile))
	}
	if target == oldFile {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, newPath)
	}
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, newPath)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(oldFile)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o644)
}

func (r *FileRepository) Delete(ctx context.Context, path string) error {
	if err := validateFilePath(path); err != nil {
		return err
	}
	file := filepath.Join(r.root, filepath.FromSlash(path))
	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, path)
		}
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%w: %s is a directory", domain.ErrInvalidPath, path)
	}
	return os.Remove(file)
}

func validateFilePath(path string) error {
	if path == "" || path == "." || path == ".." ||
		strings.HasPrefix(path, "/") || strings.Contains(path, "..") ||
		strings.ContainsAny(path, `\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, path)
	}
	return nil
}
