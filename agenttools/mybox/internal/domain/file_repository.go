package domain

import "context"

type FileKind string

const (
	FileKindFile FileKind = "file"
	FileKindDir  FileKind = "dir"
)

type FileEntry struct {
	Path   string
	Name   string
	Kind   FileKind
	Status string
}

type FileRepository interface {
	Tree(ctx context.Context) ([]FileEntry, error)
	Content(ctx context.Context, path string) (string, error)
	Raw(ctx context.Context, path string) ([]byte, error)
	Save(ctx context.Context, path string, content string) error
	Move(ctx context.Context, oldPath string, newPath string) error
	Copy(ctx context.Context, oldPath string, newPath string) error
	Delete(ctx context.Context, path string) error
}
