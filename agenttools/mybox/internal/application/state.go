package application

import (
	"context"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

const maxRecentFiles = 50

type StateUseCase struct {
	State domain.StateStore
}

func NewStateUseCase(state domain.StateStore) *StateUseCase {
	return &StateUseCase{State: state}
}

func (u *StateUseCase) Get(ctx context.Context) (*domain.State, error) {
	return u.State.Load(ctx)
}

func (u *StateUseCase) ToggleFavorite(ctx context.Context, path string, enabled bool) error {
	return u.State.Update(ctx, func(state *domain.State) error {
		state.Favorites = removeString(state.Favorites, path)
		if enabled {
			state.Favorites = append(state.Favorites, path)
		}
		return nil
	})
}

func (u *StateUseCase) RecordRecent(ctx context.Context, path string) error {
	return u.State.Update(ctx, func(state *domain.State) error {
		state.RecentFiles = removeString(state.RecentFiles, path)
		state.RecentFiles = append([]string{path}, state.RecentFiles...)
		if len(state.RecentFiles) > maxRecentFiles {
			state.RecentFiles = state.RecentFiles[:maxRecentFiles]
		}
		return nil
	})
}

func (u *StateUseCase) RemoveRecent(ctx context.Context, path string) error {
	return u.State.Update(ctx, func(state *domain.State) error {
		state.RecentFiles = removeString(state.RecentFiles, path)
		return nil
	})
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, v := range values {
		if v != target {
			out = append(out, v)
		}
	}
	return out
}
