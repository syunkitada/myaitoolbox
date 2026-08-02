package config

import (
	"context"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type Store struct {
	path string
}

func NewStore() *Store {
	return &Store{path: configPath()}
}

func configPath() string {
	if p := os.Getenv("MYBOX_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "mybox", "config.yaml")
}

type fileConfig struct {
	Projects       []projectEntry `yaml:"projects"`
	DefaultProject string         `yaml:"default_project"`
}

type projectEntry struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

func (s *Store) Load(ctx context.Context) (*domain.Config, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &domain.Config{}, nil
		}
		return nil, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return nil, err
	}
	cfg := &domain.Config{DefaultProject: fc.DefaultProject}
	for _, p := range fc.Projects {
		cfg.Projects = append(cfg.Projects, domain.Project{Name: p.Name, Path: p.Path})
	}
	return cfg, nil
}

func (s *Store) Save(ctx context.Context, cfg *domain.Config) error {
	fc := fileConfig{DefaultProject: cfg.DefaultProject}
	for _, p := range cfg.Projects {
		fc.Projects = append(fc.Projects, projectEntry{Name: p.Name, Path: p.Path})
	}
	data, err := yaml.Marshal(fc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
