package config

import (
	"context"
	"os"
	"path/filepath"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

type StateStore struct {
	path string
}

func NewStateStore() *StateStore {
	return &StateStore{path: filepath.Join(filepath.Dir(configPath()), "state.yaml")}
}

type fileState struct {
	Favorites   []fileFavorite `yaml:"favorites"`
	RecentFiles []string       `yaml:"recent_files"`
}

type fileFavorite struct {
	Project string `yaml:"project,omitempty"`
	Path    string `yaml:"path"`
}

// UnmarshalYAML keeps state.yaml files written before favorites stored their
// project name readable. Legacy scalar entries are resolved when metadata is
// loaded and then persisted in the object form.
func (f *fileFavorite) UnmarshalYAML(data []byte) error {
	var path string
	if err := yaml.Unmarshal(data, &path); err == nil {
		f.Path = path
		return nil
	}
	type plainFavorite fileFavorite
	var favorite plainFavorite
	if err := yaml.Unmarshal(data, &favorite); err != nil {
		return err
	}
	*f = fileFavorite(favorite)
	return nil
}

func (s *StateStore) Load(ctx context.Context) (*domain.State, error) {
	configMu.Lock()
	defer configMu.Unlock()
	return s.loadLocked(ctx)
}

func (s *StateStore) loadLocked(ctx context.Context) (*domain.State, error) {
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
	favorites := make([]domain.Favorite, 0, len(fs.Favorites))
	for _, favorite := range fs.Favorites {
		favorites = append(favorites, domain.Favorite{Project: favorite.Project, Path: favorite.Path})
	}
	return &domain.State{Favorites: favorites, RecentFiles: fs.RecentFiles}, nil
}

// Update applies fn to the persisted state under a global lock, so concurrent
// requests (e.g. favorite toggles from different projects) cannot drop each
// other's changes.
func (s *StateStore) Update(ctx context.Context, fn func(*domain.State) error) error {
	configMu.Lock()
	defer configMu.Unlock()
	state, err := s.loadLocked(ctx)
	if err != nil {
		return err
	}
	if err := fn(state); err != nil {
		return err
	}
	return s.saveLocked(ctx, state)
}

func (s *StateStore) saveLocked(ctx context.Context, state *domain.State) error {
	favorites := make([]fileFavorite, 0, len(state.Favorites))
	for _, favorite := range state.Favorites {
		favorites = append(favorites, fileFavorite{Project: favorite.Project, Path: favorite.Path})
	}
	fs := fileState{Favorites: favorites, RecentFiles: state.RecentFiles}
	if fs.Favorites == nil {
		fs.Favorites = []fileFavorite{}
	}
	if fs.RecentFiles == nil {
		fs.RecentFiles = []string{}
	}
	data, err := yaml.Marshal(fs)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, data, 0o644)
}

func (s *StateStore) Save(ctx context.Context, state *domain.State) error {
	configMu.Lock()
	defer configMu.Unlock()
	return s.saveLocked(ctx, state)
}
