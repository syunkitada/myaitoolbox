package domain

import "context"

type ConfigStore interface {
	Load(ctx context.Context) (*Config, error)
	Save(ctx context.Context, config *Config) error
}
