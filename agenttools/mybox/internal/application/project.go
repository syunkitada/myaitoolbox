package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type ProjectUseCase struct {
	Config domain.ConfigStore
}

func NewProjectUseCase(config domain.ConfigStore) *ProjectUseCase {
	return &ProjectUseCase{Config: config}
}

func (u *ProjectUseCase) List(ctx context.Context) ([]domain.Project, error) {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return nil, err
	}
	return cfg.Projects, nil
}

func (u *ProjectUseCase) Default(ctx context.Context) (string, error) {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return "", err
	}
	return cfg.DefaultProject, nil
}

func (u *ProjectUseCase) Add(ctx context.Context, path string) (*domain.Project, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	abs = filepath.Clean(abs)
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", domain.ErrInvalidPath, abs)
	}
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(abs)
	updated := false
	for i := range cfg.Projects {
		if cfg.Projects[i].Name == name {
			cfg.Projects[i].Path = abs
			updated = true
			break
		}
	}
	if !updated {
		cfg.Projects = append(cfg.Projects, domain.Project{Name: name, Path: abs})
	}
	if cfg.DefaultProject == "" {
		cfg.DefaultProject = name
	}
	if err := u.Config.Save(ctx, cfg); err != nil {
		return nil, err
	}
	return &domain.Project{Name: name, Path: abs}, nil
}

// PathCandidates returns existing directory paths matching the given prefix so
// the UI can offer real directories when registering a project. An empty prefix
// returns the subdirectories of the user's home directory; a trailing slash
// lists the subdirectories of the directory itself.
func (u *ProjectUseCase) PathCandidates(ctx context.Context, prefix string) ([]string, error) {
	_ = ctx
	prefix = strings.TrimSpace(prefix)
	listChildren := prefix == "" || strings.HasSuffix(prefix, string(filepath.Separator))
	if prefix == "" || prefix == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		prefix = home
	} else if strings.HasPrefix(prefix, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		prefix = filepath.Join(home, prefix[2:])
	}
	prefix = filepath.Clean(prefix)
	if !filepath.IsAbs(prefix) {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		prefix = filepath.Join(wd, prefix)
	}

	scanDir := prefix
	match := ""
	if !listChildren {
		scanDir = filepath.Dir(prefix)
		match = filepath.Base(prefix)
	}

	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil, nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if match != "" && !strings.HasPrefix(e.Name(), match) {
			continue
		}
		out = append(out, filepath.Join(scanDir, e.Name()))
	}
	sort.Strings(out)
	return out, nil
}

func (u *ProjectUseCase) Remove(ctx context.Context, name string) error {
	cfg, err := u.Config.Load(ctx)
	if err != nil {
		return err
	}
	idx := -1
	for i := range cfg.Projects {
		if cfg.Projects[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("%w: project %s", domain.ErrNotFound, name)
	}
	cfg.Projects = append(cfg.Projects[:idx], cfg.Projects[idx+1:]...)
	if cfg.DefaultProject == name {
		cfg.DefaultProject = ""
	}
	return u.Config.Save(ctx, cfg)
}

func validatePath(path string) error {
	if path == "" || path == "." || path == ".." ||
		strings.HasPrefix(path, "/") || strings.Contains(path, "..") ||
		strings.ContainsAny(path, `\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, path)
	}
	return nil
}
