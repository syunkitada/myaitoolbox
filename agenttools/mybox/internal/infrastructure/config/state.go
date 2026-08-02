package config

import (
	"context"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type StateStore struct {
	path string
}

func NewStateStore() *StateStore {
	return &StateStore{path: filepath.Join(filepath.Dir(configPath()), "state.yaml")}
}

type fileState struct {
	Favorites   []string `yaml:"favorites"`
	RecentFiles []string `yaml:"recent_files"`
}

func (s *StateStore) Load(ctx context.Context) (*domain.State, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return &domain.State{}, nil
		}
		return nil, err
	}
	var fs fileState
	if err := yaml.Unmarshal(data, &fs); err != nil {
		return nil, err
	}
	return &domain.State{Favorites: fs.Favorites, RecentFiles: fs.RecentFiles}, nil
}

func (s *StateStore) Save(ctx context.Context, state *domain.State) error {
	fs := fileState{Favorites: state.Favorites, RecentFiles: state.RecentFiles}
	if fs.Favorites == nil {
		fs.Favorites = []string{}
	}
	if fs.RecentFiles == nil {
		fs.RecentFiles = []string{}
	}
	data, err := yaml.Marshal(fs)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
