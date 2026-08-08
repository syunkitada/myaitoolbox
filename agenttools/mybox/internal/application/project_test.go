package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type memoryConfigStore struct {
	cfg *domain.Config
}

func (s *memoryConfigStore) Load(ctx context.Context) (*domain.Config, error) {
	return s.cfg, nil
}

func (s *memoryConfigStore) Save(ctx context.Context, cfg *domain.Config) error {
	s.cfg = cfg
	return nil
}

func TestProjectAddSetsDefault(t *testing.T) {
	store := &memoryConfigStore{cfg: &domain.Config{}}
	uc := NewProjectUseCase(store)
	ctx := context.Background()
	dir := t.TempDir()

	proj, err := uc.Add(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Base(dir), proj.Name)

	cfg, _ := store.Load(ctx)
	assert.Equal(t, proj.Name, cfg.DefaultProject)
}

func TestProjectAddInvalidDir(t *testing.T) {
	uc := NewProjectUseCase(&memoryConfigStore{cfg: &domain.Config{}})
	_, err := uc.Add(context.Background(), "/nonexistent/path/xyz")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
}

func TestProjectRemoveClearsDefault(t *testing.T) {
	store := &memoryConfigStore{cfg: &domain.Config{}}
	uc := NewProjectUseCase(store)
	ctx := context.Background()
	dir := t.TempDir()
	proj, err := uc.Add(ctx, dir)
	require.NoError(t, err)

	require.NoError(t, uc.Remove(ctx, proj.Name))
	cfg, _ := store.Load(ctx)
	assert.Empty(t, cfg.Projects)
	assert.Empty(t, cfg.DefaultProject)
}

func TestProjectRemoveMissing(t *testing.T) {
	uc := NewProjectUseCase(&memoryConfigStore{cfg: &domain.Config{}})
	err := uc.Remove(context.Background(), "nope")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPathCandidates(t *testing.T) {
	uc := NewProjectUseCase(&memoryConfigStore{cfg: &domain.Config{}})
	ctx := context.Background()

	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "notes"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes", "deep.md"), []byte("x"), 0o644))

	// Prefix matches existing directories only.
	got, err := uc.PathCandidates(ctx, filepath.Join(root, "doc"))
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(root, "docs")}, got)

	// Trailing slash lists subdirectories.
	got, err = uc.PathCandidates(ctx, filepath.Join(root, "docs")+string(filepath.Separator))
	require.NoError(t, err)
	assert.Empty(t, got)

	// Non-existent prefix yields no candidates.
	got, err = uc.PathCandidates(ctx, filepath.Join(root, "missing"))
	require.NoError(t, err)
	assert.Empty(t, got)

	// Files and hidden entries are never suggested.
	got, err = uc.PathCandidates(ctx, filepath.Join(root, "notes"))
	require.NoError(t, err)
	assert.Equal(t, []string{filepath.Join(root, "notes")}, got)
}

