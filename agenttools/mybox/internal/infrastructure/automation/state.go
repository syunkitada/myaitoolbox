package automation

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
	"golang.org/x/sys/unix"
)

type RunStore struct {
	root string
}

func NewRunStore(root string) *RunStore {
	return &RunStore{root: root}
}

func (s *RunStore) Find(_ context.Context, project, triggerID, eventID string) (*domain.AutomationRun, error) {
	path := s.runPath(project, triggerID, eventID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: automation run %s", domain.ErrNotFound, eventID)
		}
		return nil, err
	}
	var run domain.AutomationRun
	if err := yaml.Unmarshal(data, &run); err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *RunStore) Save(_ context.Context, run domain.AutomationRun) error {
	if strings.TrimSpace(run.Project) == "" || strings.TrimSpace(run.TriggerID) == "" || strings.TrimSpace(run.EventID) == "" {
		return fmt.Errorf("%w: run identity is incomplete", domain.ErrInvalidArgument)
	}
	data, err := yaml.Marshal(run)
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.runPath(run.Project, run.TriggerID, run.EventID), data, 0o644)
}

func (s *RunStore) List(_ context.Context, project, triggerID string) ([]domain.AutomationRun, error) {
	dir := filepath.Join(s.root, "runs", safeComponent(project), safeComponent(triggerID))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []domain.AutomationRun{}, nil
		}
		return nil, err
	}
	var runs []domain.AutomationRun
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var run domain.AutomationRun
		if err := yaml.Unmarshal(data, &run); err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.Before(runs[j].StartedAt)
	})
	return runs, nil
}

func (s *RunStore) runPath(project, triggerID, eventID string) string {
	hash := sha256.Sum256([]byte(eventID))
	name := hex.EncodeToString(hash[:]) + ".yaml"
	return filepath.Join(s.root, "runs", safeComponent(project), safeComponent(triggerID), name)
}

func safeComponent(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "_"
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

// AcquireProjectLock prevents two automation daemons or run-once commands
// from processing the same project concurrently.
func AcquireProjectLock(root, project string) (func(), error) {
	path := filepath.Join(root, "locks", safeComponent(project)+".lock")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if err == unix.EWOULDBLOCK || err == unix.EAGAIN {
			return nil, fmt.Errorf("%w: project %s", domain.ErrAutomationBusy, project)
		}
		return nil, err
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
