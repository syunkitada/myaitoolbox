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
		Favorites:   append([]domain.Favorite(nil), s.state.Favorites...),
		RecentFiles: append([]string(nil), s.state.RecentFiles...),
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
