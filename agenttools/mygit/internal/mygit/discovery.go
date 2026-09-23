package mygit

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

func DiscoverWorkspaces(root string) ([]Workspace, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve discovery root: %w", err)
	}
	absRoot = filepath.Clean(absRoot)
	if info, err := os.Stat(absRoot); err != nil {
		return nil, fmt.Errorf("stat discovery root: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("discovery root %s is not a directory", absRoot)
	}

	workspaces := make([]Workspace, 0)
	excluded := []string{filepath.Join(absRoot, DefaultRepoDir)}
	err = filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && path != absRoot {
			if entry.Name() == ".git" || isExcluded(path, excluded) {
				return filepath.SkipDir
			}
		}
		if !entry.IsDir() {
			return nil
		}

		manifestPath := filepath.Join(path, ManifestFilename)
		manifestInfo, err := os.Stat(manifestPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("stat %s: %w", manifestPath, err)
		}
		if manifestInfo.IsDir() {
			return fmt.Errorf("manifest path %s is a directory", manifestPath)
		}
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			return err
		}
		workspace, err := BuildWorkspace(path, manifest, manifestPath)
		if err != nil {
			return fmt.Errorf("Workspace %s: %w", path, err)
		}
		workspaces = append(workspaces, workspace)
		for _, target := range workspace.Targets {
			excluded = append(excluded, target.Path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover Workspaces: %w", err)
	}
	if len(workspaces) == 0 {
		return nil, ErrNoWorkspace
	}
	sort.Slice(workspaces, func(i, j int) bool { return workspaces[i].Root < workspaces[j].Root })
	if err := ValidateTargetCollisions(workspaces); err != nil {
		return nil, err
	}
	return workspaces, nil
}

func isExcluded(path string, excluded []string) bool {
	for _, root := range excluded {
		if isWithin(root, path) {
			return true
		}
	}
	return false
}
