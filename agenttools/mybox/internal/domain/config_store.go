package domain

import "context"

type ConfigStore interface {
	Load(ctx context.Context) (*Config, error)
	Save(ctx context.Context, config *Config) error
	// Update executes fn atomically on the persisted config, so that
	// concurrent read-modify-write cycles cannot drop changes.
	Update(ctx context.Context, fn func(*Config) error) error
}
