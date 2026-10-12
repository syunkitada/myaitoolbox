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

func (u *StateUseCase) RecordRecent(ctx context.Context, path, project string) error {
	return u.State.Update(ctx, func(state *domain.State) error {
		recentFiles := recentFilesForProject(state, project)
		recentFiles = removeString(recentFiles, path)
		recentFiles = append([]string{path}, recentFiles...)
		if len(recentFiles) > maxRecentFiles {
			recentFiles = recentFiles[:maxRecentFiles]
		}
		state.RecentFilesByProject[strings.TrimSpace(project)] = recentFiles
		return nil
	})
}

// RecentFiles returns recent files for one project. The empty project key is
// reserved for state written by older versions, whose recent files were not
// project-scoped. On first access, legacy entries are assigned to the current
// project and persisted as project-scoped state.
func (u *StateUseCase) RecentFiles(ctx context.Context, project string) ([]string, error) {
	project = strings.TrimSpace(project)
	var recentFiles []string
	err := u.State.Update(ctx, func(state *domain.State) error {
		migrateLegacyRecentFiles(state, project)
		recentFiles = append([]string(nil), state.RecentFilesByProject[project]...)
		return nil
	})
	return recentFiles, err
}

// MovePath rewrites favorite and recent-file paths after a project-relative
// path is moved. Project-scoped favorites are only rewritten for the moved
// project; legacy favorites without a project remain eligible for migration.
func (u *StateUseCase) MovePath(ctx context.Context, oldPath, newPath, project string) error {
	oldPath = pathpkg.Clean(oldPath)
	newPath = pathpkg.Clean(newPath)
	if oldPath == newPath {
		return nil
	}
	return u.State.Update(ctx, func(state *domain.State) error {
		for i := range state.Favorites {
			favorite := &state.Favorites[i]
			if !pathIsUnder(favorite.Path, oldPath) {
				continue
			}
			if favorite.Project != "" && favorite.Project != project {
				continue
			}
			favorite.Path = movedPath(favorite.Path, oldPath, newPath)
		}
		project = strings.TrimSpace(project)
		migrateLegacyRecentFiles(state, project)
		recentFiles := recentFilesForProject(state, project)
		for i, recent := range recentFiles {
			if pathIsUnder(recent, oldPath) {
				recentFiles[i] = movedPath(recent, oldPath, newPath)
			}
		}
		state.RecentFilesByProject[strings.TrimSpace(project)] = recentFiles
		return nil
	})
}

func (u *StateUseCase) RemoveRecent(ctx context.Context, path, project string) error {
	return u.State.Update(ctx, func(state *domain.State) error {
		project = strings.TrimSpace(project)
		state.RecentFilesByProject[project] = removeString(recentFilesForProject(state, project), path)
		return nil
	})
}

func recentFilesForProject(state *domain.State, project string) []string {
	if state.RecentFilesByProject == nil {
		state.RecentFilesByProject = map[string][]string{}
	}
	project = strings.TrimSpace(project)
	return state.RecentFilesByProject[project]
}

func mergeRecentFiles(current, legacy []string) []string {
	merged := append([]string(nil), current...)
	for _, path := range legacy {
		if !containsString(merged, path) {
			merged = append(merged, path)
		}
	}
	if len(merged) > maxRecentFiles {
		merged = merged[:maxRecentFiles]
	}
	return merged
}

func migrateLegacyRecentFiles(state *domain.State, project string) {
	if project == "" {
		return
	}
	recentFilesForProject(state, "")
	legacy := state.RecentFilesByProject[""]
	if len(legacy) == 0 {
		return
	}
	state.RecentFilesByProject[project] = mergeRecentFiles(state.RecentFilesByProject[project], legacy)
	delete(state.RecentFilesByProject, "")
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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

func pathIsUnder(candidate, parent string) bool {
	candidate = pathpkg.Clean(candidate)
	return candidate == parent || strings.HasPrefix(candidate, parent+"/")
}

func movedPath(path, oldPath, newPath string) string {
	suffix := strings.TrimPrefix(pathpkg.Clean(path), oldPath)
	return pathpkg.Join(newPath, suffix)
}
