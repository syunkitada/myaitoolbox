package infrastructure

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/syunkitada/myaitoolbox/agentrun/internal/domain"
	"gopkg.in/yaml.v3"
)

// TaskStore implements domain.TaskStore using the filesystem.
type TaskStore struct {
	workspaceRoot string
}

// NewTaskStore creates a TaskStore for the given workspace root.
func NewTaskStore(workspaceRoot string) *TaskStore {
	return &TaskStore{workspaceRoot: workspaceRoot}
}

func (s *TaskStore) tasksDir() string {
	return filepath.Join(s.workspaceRoot, "tasks")
}

func (s *TaskStore) archiveDir() string {
	return filepath.Join(s.workspaceRoot, "archive")
}

// EnsureWorkspace creates the workspace directory structure.
func (s *TaskStore) EnsureWorkspace() error {
	for _, d := range []string{"tasks", "archive", "events"} {
		if err := os.MkdirAll(filepath.Join(s.workspaceRoot, d), 0755); err != nil {
			return fmt.Errorf("failed to create workspace dir %s/: %w", d, err)
		}
	}
	return nil
}

// ListTaskDirs returns all task directories.
func (s *TaskStore) ListTaskDirs() ([]string, error) {
	return s.listTaskDirsFiltered(nil)
}

// ListTaskDirsByStatus returns task directories filtered by status.
func (s *TaskStore) ListTaskDirsByStatus(status domain.TaskStatus) ([]*domain.TaskInfo, error) {
	var result []*domain.TaskInfo
	dirs, err := s.listTaskDirsFiltered(&status)
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		meta, err := s.ReadMetadata(dir)
		if err != nil {
			continue
		}
		result = append(result, &domain.TaskInfo{
			TaskDir:  dir,
			TaskID:   filepath.Base(dir),
			Metadata: meta,
		})
	}
	return result, nil
}

func (s *TaskStore) listTaskDirsFiltered(status *domain.TaskStatus) ([]string, error) {
	entries, err := os.ReadDir(s.tasksDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if len(entry.Name()) > 0 && entry.Name()[0] == '.' {
			continue
		}
		dir := filepath.Join(s.tasksDir(), entry.Name())

		if status != nil {
			meta, err := s.ReadMetadata(dir)
			if err != nil {
				continue
			}
			if meta.Status != *status {
				continue
			}
		}
		dirs = append(dirs, dir)
	}
	return dirs, nil
}

// ReadMetadata reads metadata.yaml from a task directory.
func (s *TaskStore) ReadMetadata(taskDir string) (*domain.Metadata, error) {
	data, err := os.ReadFile(filepath.Join(taskDir, "metadata.yaml"))
	if err != nil {
		return nil, err
	}
	var meta domain.Metadata
	if err := yaml.Unmarshal(data, &meta); err != nil {
		return nil, err
	}
	meta.EnsureDefaults()
	return &meta, nil
}

// WriteMetadata writes metadata.yaml to a task directory.
func (s *TaskStore) WriteMetadata(taskDir string, meta *domain.Metadata) error {
	meta.UpdatedAt = domain.NowISO()
	data, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(taskDir, "metadata.yaml"), data, 0644)
}

// MoveToArchive moves a task directory to archive/YYYY/MM/DD/.
func (s *TaskStore) MoveToArchive(taskDir string) error {
	meta, err := s.ReadMetadata(taskDir)
	if err != nil {
		return err
	}

	// Parse created_at for archive path
	year, month, day := parseDateForArchive(meta.CreatedAt)
	archiveSubDir := filepath.Join(s.archiveDir(), year, month, day)
	if err := os.MkdirAll(archiveSubDir, 0755); err != nil {
		return err
	}

	taskName := filepath.Base(taskDir)
	dest := filepath.Join(archiveSubDir, taskName)
	return os.Rename(taskDir, dest)
}

func parseDateForArchive(isoDate string) (year, month, day string) {
	// Parse "2026-07-20T08:18:06Z" format
	if len(isoDate) >= 10 {
		parts := strings.SplitN(isoDate[:10], "-", 3)
		if len(parts) == 3 {
			return parts[0], parts[1], parts[2]
		}
	}
	return "0000", "00", "00"
}
