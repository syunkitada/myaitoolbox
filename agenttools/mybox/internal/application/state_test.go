package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type stateStoreStub struct {
	state domain.State
}

func (s *stateStoreStub) Load(context.Context) (*domain.State, error) {
	return &s.state, nil
}

func (s *stateStoreStub) Save(_ context.Context, state *domain.State) error {
	s.state = *state
	return nil
}

func (s *stateStoreStub) Update(_ context.Context, fn func(*domain.State) error) error {
	state := domain.State{
		Favorites:            append([]domain.Favorite(nil), s.state.Favorites...),
		RecentFilesByProject: cloneRecentFilesByProjectForTest(s.state.RecentFilesByProject),
	}
	if err := fn(&state); err != nil {
		return err
	}
	s.state = state
	return nil
}

func TestRemoveFavoritesRemovesDeletedDirectoryAndDescendantsForProject(t *testing.T) {
	store := &stateStoreStub{state: domain.State{Favorites: []domain.Favorite{
		{Project: "docs", Path: "notes/guide"},
		{Project: "other", Path: "notes/guide"},
		{Project: "docs", Path: "notes/guide/child.md"},
		{Project: "docs", Path: "notes/guide.md"},
		{Path: "notes/guide/child.md"},
	}}}
	useCase := NewStateUseCase(store)

	require.NoError(t, useCase.RemoveFavorites(context.Background(), "docs", "notes/guide"))

	assert.Equal(t, []domain.Favorite{
		{Project: "other", Path: "notes/guide"},
		{Project: "docs", Path: "notes/guide.md"},
	}, store.state.Favorites)
}

func TestRemoveFavoritesNormalizesDeletedPath(t *testing.T) {
	store := &stateStoreStub{state: domain.State{Favorites: []domain.Favorite{
		{Project: "docs", Path: "notes/guide.md"},
	}}}
	useCase := NewStateUseCase(store)

	require.NoError(t, useCase.RemoveFavorites(context.Background(), "docs", "notes/./guide.md"))

	assert.Empty(t, store.state.Favorites)
}

func TestMovePathRewritesFavoritesAndRecentFiles(t *testing.T) {
	store := &stateStoreStub{state: domain.State{
		Favorites: []domain.Favorite{
			{Project: "demo", Path: "_tasks/old"},
			{Project: "demo", Path: "_tasks/old/notes.md"},
			{Path: "_tasks/old/task.md"},
			{Project: "other", Path: "_tasks/old/task.md"},
			{Project: "demo", Path: "_tasks/older/task.md"},
		},
		RecentFilesByProject: map[string][]string{
			"demo":  {"_tasks/old/task.md", "_tasks/old", "_tasks/older/task.md"},
			"other": {"_tasks/old/other.md"},
		},
	}}
	useCase := NewStateUseCase(store)

	require.NoError(t, useCase.MovePath(context.Background(), "_tasks/./old", "_tasks/new", "demo"))

	assert.Equal(t, []domain.Favorite{
		{Project: "demo", Path: "_tasks/new"},
		{Project: "demo", Path: "_tasks/new/notes.md"},
		{Path: "_tasks/new/task.md"},
		{Project: "other", Path: "_tasks/old/task.md"},
		{Project: "demo", Path: "_tasks/older/task.md"},
	}, store.state.Favorites)
	assert.Equal(t, []string{"_tasks/new/task.md", "_tasks/new", "_tasks/older/task.md"}, store.state.RecentFilesByProject["demo"])
	assert.Equal(t, []string{"_tasks/old/other.md"}, store.state.RecentFilesByProject["other"])
}

func TestRecentFilesAreScopedToProject(t *testing.T) {
	store := &stateStoreStub{state: domain.State{
		RecentFilesByProject: map[string][]string{
			"demo":  {"notes/demo.md"},
			"other": {"notes/other.md"},
		},
	}}
	useCase := NewStateUseCase(store)

	require.NoError(t, useCase.RecordRecent(context.Background(), "notes/new.md", "demo"))
	assert.Equal(t, []string{"notes/new.md", "notes/demo.md"}, store.state.RecentFilesByProject["demo"])
	assert.Equal(t, []string{"notes/other.md"}, store.state.RecentFilesByProject["other"])

	require.NoError(t, useCase.RemoveRecent(context.Background(), "notes/new.md", "demo"))
	assert.Equal(t, []string{"notes/demo.md"}, store.state.RecentFilesByProject["demo"])
	assert.Equal(t, []string{"notes/other.md"}, store.state.RecentFilesByProject["other"])
}

func TestMovePathMigratesLegacyRecentFilesToProject(t *testing.T) {
	store := &stateStoreStub{state: domain.State{
		RecentFilesByProject: map[string][]string{"": {"_tasks/old/task.md"}},
	}}
	useCase := NewStateUseCase(store)

	require.NoError(t, useCase.MovePath(context.Background(), "_tasks/old", "_tasks/new", "demo"))
	assert.Equal(t, []string{"_tasks/new/task.md"}, store.state.RecentFilesByProject["demo"])
	assert.NotContains(t, store.state.RecentFilesByProject, "")
}

func cloneRecentFilesByProjectForTest(values map[string][]string) map[string][]string {
	if values == nil {
		return nil
	}
	clone := make(map[string][]string, len(values))
	for project, paths := range values {
		clone[project] = append([]string(nil), paths...)
	}
	return clone
}
