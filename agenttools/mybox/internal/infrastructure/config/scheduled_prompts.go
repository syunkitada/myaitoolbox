package config

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

// ScheduledPromptStore stores reservations beside config.yaml so they remain
// available when the Web UI or browser is closed and after mybox restarts.
type ScheduledPromptStore struct {
	path string
}

func NewScheduledPromptStore() *ScheduledPromptStore {
	return &ScheduledPromptStore{path: filepath.Join(filepath.Dir(configPath()), "scheduled-prompts.yaml")}
}

// NewScheduledPromptStoreAt is intended for isolated callers and tests.
func NewScheduledPromptStoreAt(path string) *ScheduledPromptStore {
	return &ScheduledPromptStore{path: path}
}

type scheduledPromptFile struct {
	Prompts []scheduledPromptEntry `yaml:"prompts"`
}

type scheduledPromptEntry struct {
	ID          string    `yaml:"id"`
	Project     string    `yaml:"project"`
	Target      string    `yaml:"target"`
	Text        string    `yaml:"text"`
	ScheduledAt time.Time `yaml:"scheduled_at"`
}

func (s *ScheduledPromptStore) List(ctx context.Context, project string) ([]domain.ScheduledPrompt, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	configMu.Lock()
	defer configMu.Unlock()
	return s.listLocked(project)
}

func (s *ScheduledPromptStore) listLocked(project string) ([]domain.ScheduledPrompt, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.ScheduledPrompt{}, nil
		}
		return nil, err
	}
	var file scheduledPromptFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	prompts := make([]domain.ScheduledPrompt, 0, len(file.Prompts))
	for _, entry := range file.Prompts {
		if project != "" && entry.Project != project {
			continue
		}
		prompts = append(prompts, domain.ScheduledPrompt{
			ID:          entry.ID,
			Project:     entry.Project,
			Target:      entry.Target,
			Text:        entry.Text,
			ScheduledAt: entry.ScheduledAt,
		})
	}
	sort.SliceStable(prompts, func(i, j int) bool {
		if prompts[i].ScheduledAt.Equal(prompts[j].ScheduledAt) {
			return prompts[i].ID < prompts[j].ID
		}
		return prompts[i].ScheduledAt.Before(prompts[j].ScheduledAt)
	})
	return prompts, nil
}

func (s *ScheduledPromptStore) Create(ctx context.Context, prompt domain.ScheduledPrompt) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	configMu.Lock()
	defer configMu.Unlock()
	current, err := s.listLocked("")
	if err != nil {
		return err
	}
	for _, existing := range current {
		if existing.ID == prompt.ID {
			return domain.ErrAlreadyExists
		}
	}
	current = append(current, prompt)
	return s.saveLocked(current)
}

func (s *ScheduledPromptStore) Delete(ctx context.Context, project, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	configMu.Lock()
	defer configMu.Unlock()
	current, err := s.listLocked("")
	if err != nil {
		return err
	}
	next := make([]domain.ScheduledPrompt, 0, len(current))
	found := false
	for _, prompt := range current {
		if prompt.ID == id && prompt.Project == project {
			found = true
			continue
		}
		next = append(next, prompt)
	}
	if !found {
		return domain.ErrNotFound
	}
	return s.saveLocked(next)
}

func (s *ScheduledPromptStore) saveLocked(prompts []domain.ScheduledPrompt) error {
	entries := make([]scheduledPromptEntry, 0, len(prompts))
	for _, prompt := range prompts {
		entries = append(entries, scheduledPromptEntry{
			ID:          prompt.ID,
			Project:     prompt.Project,
			Target:      prompt.Target,
			Text:        prompt.Text,
			ScheduledAt: prompt.ScheduledAt,
		})
	}
	data, err := yaml.Marshal(scheduledPromptFile{Prompts: entries})
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.path, data, 0o644)
}
