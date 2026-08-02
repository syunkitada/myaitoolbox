package domain

import "context"

type KnowledgeRepository interface {
	List(ctx context.Context) ([]Knowledge, error)
	Find(ctx context.Context, path string) (*Knowledge, error)
	RawContent(ctx context.Context, path string) (string, error)
	Create(ctx context.Context, path string, content string) error
	SaveContent(ctx context.Context, path string, content string) error
	Move(ctx context.Context, oldPath string, newPath string) error
	Rename(ctx context.Context, oldPath string, newName string) error
}
