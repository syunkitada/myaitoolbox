package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestStateStoreLoadsLegacyPathOnlyFavorites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	require.NoError(t, os.WriteFile(path, []byte("favorites:\n  - notes/n1\nrecent_files: []\n"), 0o644))

	state, err := (&StateStore{path: path}).Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []domain.Favorite{{Path: "notes/n1"}}, state.Favorites)
}

func TestStateStoreSavesFavoriteProject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")
	store := &StateStore{path: path}
	require.NoError(t, store.Save(context.Background(), &domain.State{
		Favorites: []domain.Favorite{{Project: "demo", Path: "notes/n1"}},
	}))

	state, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []domain.Favorite{{Project: "demo", Path: "notes/n1"}}, state.Favorites)
}
