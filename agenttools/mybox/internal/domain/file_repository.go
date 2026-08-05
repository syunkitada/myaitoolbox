package domain

import "context"

type FileKind string

const (
	FileKindFile FileKind = "file"
	FileKindDir  FileKind = "dir"
)

type FileEntry struct {
	Path string
	Name string
	Kind FileKind
}

type FileRepository interface {
	Tree(ctx context.Context) ([]FileEntry, error)
	Content(ctx context.Context, path string) (string, error)
	Save(ctx context.Context, path string, content string) error
	Move(ctx context.Context, oldPath string, newPath string) error
}
