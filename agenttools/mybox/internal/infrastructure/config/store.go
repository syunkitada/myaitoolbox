package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

// configMu serializes read-modify-write cycles on config.yaml and state.yaml.
// Both stores back onto files shared by every App instance (one per project),
// so the lock lives at the package level to protect cross-instance updates.
var configMu sync.Mutex

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
	configMu.Lock()
	defer configMu.Unlock()
	return s.loadLocked(ctx)
}

func (s *Store) loadLocked(ctx context.Context) (*domain.Config, error) {
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

// Update applies fn to the persisted config under a global lock, so
// concurrent read-modify-write cycles from different apps cannot drop each
// other's changes.
func (s *Store) Update(ctx context.Context, fn func(*domain.Config) error) error {
	configMu.Lock()
	defer configMu.Unlock()
	cfg, err := s.loadLocked(ctx)
	if err != nil {
		return err
	}
	if err := fn(cfg); err != nil {
		return err
	}
	return s.saveLocked(ctx, cfg)
}

func (s *Store) saveLocked(ctx context.Context, cfg *domain.Config) error {
	fc := fileConfig{DefaultProject: cfg.DefaultProject}
	for _, p := range cfg.Projects {
		fc.Projects = append(fc.Projects, projectEntry{Name: p.Name, Path: p.Path})
	}
	data, err := yaml.Marshal(fc)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, data, 0o644)
}

func (s *Store) Save(ctx context.Context, cfg *domain.Config) error {
	configMu.Lock()
	defer configMu.Unlock()
	return s.saveLocked(ctx, cfg)
}
