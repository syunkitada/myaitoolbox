package domain

import "context"

type TaskRepository interface {
	List(ctx context.Context) ([]Task, error)
	Find(ctx context.Context, id string) (*Task, error)
	Create(ctx context.Context, id string, content string) error
	Rename(ctx context.Context, oldID string, newID string) error
	Update(ctx context.Context, task Task) error
	Archive(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}
