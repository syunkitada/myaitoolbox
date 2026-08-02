package markdown

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type TaskRepository struct {
	root string
}

func NewTaskRepository(root string) *TaskRepository {
	return &TaskRepository{root: root}
}

type taskFields struct {
	ID       string    `yaml:"id"`
	Title    string    `yaml:"title"`
	Status   string    `yaml:"status"`
	Priority string    `yaml:"priority"`
	Assignee string    `yaml:"assignee"`
	Due      string    `yaml:"due"`
	Tags     []string  `yaml:"tags"`
	Project  string    `yaml:"project"`
	Created  time.Time `yaml:"created"`
}

func (r *TaskRepository) List(ctx context.Context) ([]domain.Task, error) {
	return r.list(filepath.Join(r.root, "tasks"), false)
}

func (r *TaskRepository) ListArchived(ctx context.Context) ([]domain.Task, error) {
	return r.list(filepath.Join(r.root, "archives", "tasks"), true)
}

func (r *TaskRepository) list(dir string, archived bool) ([]domain.Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var tasks []domain.Task
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := r.readTask(filepath.Join(dir, e.Name()), e.Name(), archived)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		tasks = append(tasks, *t)
	}
	return tasks, nil
}

func (r *TaskRepository) readTask(dir string, id string, archived bool) (*domain.Task, error) {
	content, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		return nil, err
	}
	fmStr, body, ok := splitFrontMatter(string(content))
	var f taskFields
	if ok {
		if err := yaml.Unmarshal([]byte(fmStr), &f); err != nil {
			return nil, err
		}
	}
	if f.ID == "" {
		f.ID = id
	}
	if f.Title == "" {
		f.Title = id
	}
	if f.Status == "" {
		f.Status = string(domain.TaskStatusTodo)
	}
	if f.Priority == "" {
		f.Priority = string(domain.TaskPriorityMedium)
	}
	return &domain.Task{
		ID:       f.ID,
		Title:    f.Title,
		Status:   domain.TaskStatus(f.Status),
		Priority: domain.TaskPriority(f.Priority),
		Assignee: f.Assignee,
		Due:      f.Due,
		Tags:     f.Tags,
		Project:  f.Project,
		Created:  f.Created,
		Body:     strings.TrimPrefix(body, "\n"),
		Archived: archived,
	}, nil
}

func (r *TaskRepository) Find(ctx context.Context, id string) (*domain.Task, error) {
	if err := validateTaskID(id); err != nil {
		return nil, err
	}
	active := filepath.Join(r.root, "tasks", id, "task.md")
	if _, err := os.Stat(active); err == nil {
		return r.readTask(filepath.Join(r.root, "tasks", id), id, false)
	}
	archived := filepath.Join(r.root, "archives", "tasks", id, "task.md")
	if _, err := os.Stat(archived); err == nil {
		return r.readTask(filepath.Join(r.root, "archives", "tasks", id), id, true)
	}
	return nil, fmt.Errorf("%w: task %s", domain.ErrNotFound, id)
}

func (r *TaskRepository) Create(ctx context.Context, id string, content string) error {
	if err := validateTaskID(id); err != nil {
		return err
	}
	dir := filepath.Join(r.root, "tasks", id)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, id)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "task.md"), []byte(content), 0o644)
}

func (r *TaskRepository) Update(ctx context.Context, task domain.Task) error {
	if err := validateTaskID(task.ID); err != nil {
		return err
	}
	path := filepath.Join(r.root, "tasks", task.ID, "task.md")
	if task.Archived {
		path = filepath.Join(r.root, "archives", "tasks", task.ID, "task.md")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, task.ID)
		}
		return err
	}
	fmStr, body, ok := splitFrontMatter(string(content))
	fm := map[string]any{}
	if ok {
		fm, err = parseFrontMatter(fmStr)
		if err != nil {
			return err
		}
	}
	fm["id"] = task.ID
	fm["title"] = task.Title
	fm["status"] = string(task.Status)
	fm["priority"] = string(task.Priority)
	fm["assignee"] = task.Assignee
	fm["due"] = task.Due
	if task.Tags == nil {
		task.Tags = []string{}
	}
	fm["tags"] = task.Tags
	fm["project"] = task.Project
	if !task.Created.IsZero() {
		fm["created"] = task.Created
	}
	if task.Body != "" {
		body = task.Body
	}
	out, err := buildFrontMatter(fm, body)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

func (r *TaskRepository) Archive(ctx context.Context, id string) error {
	if err := validateTaskID(id); err != nil {
		return err
	}
	src := filepath.Join(r.root, "tasks", id)
	dst := filepath.Join(r.root, "archives", "tasks", id)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, id)
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if isGitRepo(r.root) {
		return runGit(ctx, r.root, "mv", src, dst)
	}
	return os.Rename(src, dst)
}

func validateTaskID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, id)
	}
	return nil
}

func isGitRepo(dir string) bool {
	if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
		return true
	}
	return false
}

func runGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	return cmd.Run()
}
