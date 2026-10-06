package application

import (
	"context"
	pathpkg "path"
	"strings"

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

func (u *StateUseCase) ToggleFavorite(ctx context.Context, path, project string, enabled bool) error {
	return u.State.Update(ctx, func(state *domain.State) error {
		state.Favorites = removeFavorite(state.Favorites, path, project, enabled)
		if enabled {
			state.Favorites = append(state.Favorites, domain.Favorite{Project: project, Path: path})
		}
		return nil
	})
}

func (u *StateUseCase) ResolveFavoriteProjects(ctx context.Context, projects map[string]string) error {
	if len(projects) == 0 {
		return nil
	}
	return u.State.Update(ctx, func(state *domain.State) error {
		for i := range state.Favorites {
			favorite := &state.Favorites[i]
			if favorite.Project == "" {
				favorite.Project = projects[favorite.Path]
			}
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

func (u *StateUseCase) RemoveFavorites(ctx context.Context, project, path string) error {
	path = pathpkg.Clean(path)
	return u.State.Update(ctx, func(state *domain.State) error {
		state.Favorites = removeFavoritesUnderPath(state.Favorites, project, path)
		return nil
	})
}

func removeFavorite(values []domain.Favorite, path, project string, enabled bool) []domain.Favorite {
	out := values[:0]
	for _, favorite := range values {
		if favorite.Path != path {
			out = append(out, favorite)
			continue
		}
		if enabled {
			// Re-favoriting a legacy path upgrades it to the current project.
			if favorite.Project != "" && favorite.Project != project {
				out = append(out, favorite)
			}
			continue
		}
		if project != "" && favorite.Project != "" && favorite.Project != project {
			out = append(out, favorite)
		}
	}
	return out
}

func removeString(values []string, target string) []string {
	out := values[:0]
	for _, value := range values {
		if value != target {
			out = append(out, value)
		}
	}
	return out
}

func removeFavoritesUnderPath(values []domain.Favorite, project, path string) []domain.Favorite {
	out := values[:0]
	for _, favorite := range values {
		isUnderPath := favorite.Path == path || strings.HasPrefix(favorite.Path, path+"/")
		if !isUnderPath || (favorite.Project != "" && favorite.Project != project) {
			out = append(out, favorite)
		}
	}
	return out
}
