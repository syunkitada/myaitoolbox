package automation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron/v3"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func CronEventsBetween(ctx context.Context, def domain.TriggerDefinition, after, now time.Time) ([]domain.TriggerEvent, error) {
	if def.Type != domain.TriggerTypeCron {
		return nil, fmt.Errorf("%w: trigger %s is not cron", domain.ErrInvalidArgument, def.ID)
	}
	loc, err := time.LoadLocation(def.Timezone)
	if err != nil {
		return nil, fmt.Errorf("%w: timezone %q: %v", domain.ErrInvalidArgument, def.Timezone, err)
	}
	schedule, err := cron.ParseStandard(def.Cron)
	if err != nil {
		return nil, fmt.Errorf("%w: cron %q: %v", domain.ErrInvalidArgument, def.Cron, err)
	}
	start := after.In(loc)
	end := now.In(loc)
	if !end.After(start) {
		return nil, nil
	}
	var events []domain.TriggerEvent
	for next := schedule.Next(start); !next.After(end); next = schedule.Next(next) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		events = append(events, domain.TriggerEvent{
			ID:          def.ID + "@" + next.Format(time.RFC3339),
			TriggerID:   def.ID,
			OccurredAt:  now,
			ScheduledAt: next,
		})
	}
	return events, nil
}

func ScanFileEvents(ctx context.Context, def domain.TriggerDefinition, now time.Time) ([]domain.TriggerEvent, error) {
	if def.Type != domain.TriggerTypeFileCreated {
		return nil, fmt.Errorf("%w: trigger %s is not file_created", domain.ErrInvalidArgument, def.ID)
	}
	entries, err := os.ReadDir(def.WatchPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.TriggerEvent{}, nil
		}
		return nil, err
	}
	pattern := def.Pattern
	if pattern == "" {
		pattern = "*"
	}
	var events []domain.TriggerEvent
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if entry.IsDir() {
			continue
		}
		matched, err := filepath.Match(pattern, entry.Name())
		if err != nil {
			return nil, fmt.Errorf("%w: file pattern %q: %v", domain.ErrInvalidArgument, pattern, err)
		}
		if !matched {
			continue
		}
		path := filepath.Join(def.WatchPath, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.Mode().IsRegular() || (def.SettleFor > 0 && now.Sub(info.ModTime()) < def.SettleFor) {
			continue
		}
		event, err := fileEvent(def, path, info, now)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].SourcePath < events[j].SourcePath })
	return events, nil
}

func fileEvent(def domain.TriggerDefinition, path string, info os.FileInfo, now time.Time) (domain.TriggerEvent, error) {
	rel, err := filepath.Rel(def.ProjectRoot, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return domain.TriggerEvent{}, fmt.Errorf("%w: source path %s", domain.ErrInvalidPath, path)
	}
	rel = filepath.ToSlash(rel)
	return domain.TriggerEvent{
		ID:         fmt.Sprintf("file:%s:%s:%d:%d", def.ID, rel, info.Size(), info.ModTime().UnixNano()),
		TriggerID:  def.ID,
		OccurredAt: now,
		SourcePath: rel,
	}, nil
}

type FileWatcher struct {
	definitions []domain.TriggerDefinition
}

func NewFileWatcher(definitions []domain.TriggerDefinition) *FileWatcher {
	return &FileWatcher{definitions: append([]domain.TriggerDefinition(nil), definitions...)}
}

func (w *FileWatcher) Events(ctx context.Context) (<-chan domain.TriggerEvent, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	byPath := make(map[string][]domain.TriggerDefinition)
	for _, def := range w.definitions {
		if !def.Enabled || def.Type != domain.TriggerTypeFileCreated {
			continue
		}
		watchPath := filepath.Clean(def.WatchPath)
		if err := os.MkdirAll(def.WatchPath, 0o755); err != nil {
			_ = watcher.Close()
			return nil, err
		}
		if len(byPath[watchPath]) == 0 {
			if err := watcher.Add(def.WatchPath); err != nil {
				_ = watcher.Close()
				return nil, err
			}
		}
		byPath[watchPath] = append(byPath[watchPath], def)
	}

	out := make(chan domain.TriggerEvent)
	go func() {
		defer close(out)
		defer func() { _ = watcher.Close() }()
		pending := make(map[string]time.Time)
		seen := make(map[string]struct{})
		now := time.Now()
		for _, definitions := range byPath {
			for _, def := range definitions {
				events, scanErr := ScanFileEvents(context.Background(), def, now)
				if scanErr != nil {
					continue
				}
				for _, event := range events {
					seen[event.ID] = struct{}{}
					select {
					case out <- event:
					case <-ctx.Done():
						return
					}
				}
			}
		}

		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case err, ok := <-watcher.Errors:
				if !ok || err != nil {
					if !ok {
						return
					}
				}
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				if _, ok := byPath[filepath.Clean(filepath.Dir(event.Name))]; ok {
					pending[event.Name] = time.Now().Add(250 * time.Millisecond)
				}
			case now := <-ticker.C:
				for path, due := range pending {
					if now.Before(due) {
						continue
					}
					delete(pending, path)
					definitions := byPath[filepath.Clean(filepath.Dir(path))]
					for _, def := range definitions {
						info, statErr := os.Lstat(path)
						if statErr != nil || !info.Mode().IsRegular() {
							continue
						}
						if def.SettleFor > 0 && now.Sub(info.ModTime()) < def.SettleFor {
							next := now.Add(def.SettleFor)
							if current, exists := pending[path]; !exists || next.After(current) {
								pending[path] = next
							}
							continue
						}
						matched, matchErr := filepath.Match(def.Pattern, filepath.Base(path))
						if matchErr != nil || !matched {
							continue
						}
						event, eventErr := fileEvent(def, path, info, now)
						if eventErr != nil {
							continue
						}
						if _, exists := seen[event.ID]; exists {
							continue
						}
						seen[event.ID] = struct{}{}
						select {
						case out <- event:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()
	return out, nil
}
