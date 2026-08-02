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

func TestLoadMissingReturnsEmpty(t *testing.T) {
	store := &Store{path: filepath.Join(t.TempDir(), "config.yaml")}
	cfg, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Empty(t, cfg.Projects)
	assert.Empty(t, cfg.DefaultProject)
}

func TestSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := &Store{path: path}
	cfg := &domain.Config{
		Projects: []domain.Project{
			{Name: "a", Path: "/tmp/a"},
			{Name: "b", Path: "/tmp/b"},
		},
		DefaultProject: "a",
	}
	require.NoError(t, store.Save(context.Background(), cfg))

	got, err := store.Load(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "a", got.DefaultProject)
	require.Len(t, got.Projects, 2)
	assert.Equal(t, "/tmp/a", got.Projects[0].Path)
}

func TestSaveCreatesParentDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	store := &Store{path: filepath.Join(dir, "config.yaml")}
	require.NoError(t, store.Save(context.Background(), &domain.Config{}))
	_, err := os.Stat(filepath.Join(dir, "config.yaml"))
	require.NoError(t, err)
}
