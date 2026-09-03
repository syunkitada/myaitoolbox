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
	Title         string   `yaml:"title"`
	Status        string   `yaml:"status"`
	Priority      string   `yaml:"priority"`
	Type          string   `yaml:"type"`
	Assignee      string   `yaml:"assignee"`
	Due           string   `yaml:"due"`
	PendingUntil  string   `yaml:"pending_until"`
	PendingReason string   `yaml:"pending_reason"`
	Tags          []string `yaml:"tags"`
}

func (r *TaskRepository) List(ctx context.Context) ([]domain.Task, error) {
	tasks, err := r.list(filepath.Join(r.root, "tasks"), false)
	if err != nil {
		return nil, err
	}
	adhoc, err := r.listAdhoc(filepath.Join(r.root, "tasks", "adhoc"), false)
	if err != nil {
		return nil, err
	}
	return append(tasks, adhoc...), nil
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

func (r *TaskRepository) listAdhoc(dir string, archived bool) ([]domain.Task, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var tasks []domain.Task
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		id := strings.TrimSuffix(name, ".md")
		t, err := r.readAdhocTask(filepath.Join(dir, name), id, archived)
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
	if f.Title == "" {
		f.Title = id
	}
	if f.Status == "" {
		f.Status = string(domain.TaskStatusTodo)
	}
	if f.Priority == "" {
		f.Priority = string(domain.TaskPriorityMedium)
	}
	taskType := domain.TaskType(f.Type)
	if taskType == "" {
		taskType = domain.TaskTypeRegular
	}
	return &domain.Task{
		ID:            id,
		Title:         f.Title,
		Status:        domain.TaskStatus(f.Status),
		Priority:      domain.TaskPriority(f.Priority),
		Type:          taskType,
		Assignee:      f.Assignee,
		Due:           f.Due,
		PendingUntil:  f.PendingUntil,
		PendingReason: f.PendingReason,
		Tags:          f.Tags,
		Created:       createdFromTaskID(id),
		Body:          strings.TrimPrefix(body, "\n"),
		Archived:      archived,
	}, nil
}

func (r *TaskRepository) readAdhocTask(path string, id string, archived bool) (*domain.Task, error) {
	content, err := os.ReadFile(path)
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
		ID:            id,
		Title:         f.Title,
		Status:        domain.TaskStatus(f.Status),
		Priority:      domain.TaskPriority(f.Priority),
		Type:          domain.TaskTypeAdhoc,
		Assignee:      f.Assignee,
		Due:           f.Due,
		PendingUntil:  f.PendingUntil,
		PendingReason: f.PendingReason,
		Tags:          f.Tags,
		Created:       createdFromTaskID(id),
		Body:          strings.TrimPrefix(body, "\n"),
		Archived:      archived,
	}, nil
}

// createdFromTaskID derives the creation time from the leading YYYYMMDD
// date prefix of the task directory name (e.g. 20260801_fix-login).
func createdFromTaskID(id string) time.Time {
	if len(id) < 8 {
		return time.Time{}
	}
	t, err := time.ParseInLocation("20060102", id[:8], time.Local)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (r *TaskRepository) Find(ctx context.Context, id string) (*domain.Task, error) {
	if err := validateTaskID(id); err != nil {
		return nil, err
	}
	active := filepath.Join(r.root, "tasks", id, "task.md")
	if _, err := os.Stat(active); err == nil {
		return r.readTask(filepath.Join(r.root, "tasks", id), id, false)
	}
	adhoc := filepath.Join(r.root, "tasks", "adhoc", id+".md")
	if _, err := os.Stat(adhoc); err == nil {
		return r.readAdhocTask(adhoc, id, false)
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

func (r *TaskRepository) CreateAdhoc(ctx context.Context, id string, content string) error {
	if err := validateTaskID(id); err != nil {
		return err
	}
	adhocDir := filepath.Join(r.root, "tasks", "adhoc")
	path := filepath.Join(adhocDir, id+".md")
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, id)
	}
	if err := os.MkdirAll(adhocDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func (r *TaskRepository) Update(ctx context.Context, task domain.Task) error {
	if err := validateTaskID(task.ID); err != nil {
		return err
	}
	path := filepath.Join(r.root, "tasks", task.ID, "task.md")
	if task.Archived {
		path = filepath.Join(r.root, "archives", "tasks", task.ID, "task.md")
	}
	if _, err := os.Stat(path); err != nil {
		if task.Type == domain.TaskTypeAdhoc {
			path = filepath.Join(r.root, "tasks", "adhoc", task.ID+".md")
		}
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
	delete(fm, "id")
	delete(fm, "project")
	delete(fm, "created")
	delete(fm, "created_at")
	fm["title"] = task.Title
	fm["status"] = string(task.Status)
	fm["priority"] = string(task.Priority)
	if task.Type != "" {
		fm["type"] = string(task.Type)
	}
	fm["assignee"] = task.Assignee
	fm["due"] = task.Due
	if task.Tags == nil {
		task.Tags = []string{}
	}
	fm["tags"] = task.Tags
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
	if _, err := os.Stat(src); err != nil {
		adhoc := filepath.Join(r.root, "tasks", "adhoc", id+".md")
		if _, aerr := os.Stat(adhoc); aerr == nil {
			return fmt.Errorf("%w: adhoc tasks cannot be archived; use a regular task", domain.ErrInvalidArgument)
		}
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, id)
		}
		return err
	}
	dst := filepath.Join(r.root, "archives", "tasks", id)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	// git mv は非追跡ファイル（adhocタスクや未コミットの変更）を含むディレクトリで
	// "source directory is empty" 等で失敗するため、os.Rename + git add -A（全体）で
	// 追跡・非追跡どちらでも移動を正しくステージする。
	if isGitRepo(r.root) {
		return runGit(ctx, r.root, "add", "-A")
	}
	return nil
}

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	if err := validateTaskID(id); err != nil {
		return err
	}
	regular := filepath.Join(r.root, "tasks", id)
	adhoc := filepath.Join(r.root, "tasks", "adhoc", id+".md")
	archivedRegular := filepath.Join(r.root, "archives", "tasks", id)
	archivedAdhoc := filepath.Join(r.root, "archives", "tasks", "adhoc", id+".md")

	targets := []string{regular, adhoc, archivedRegular, archivedAdhoc}
	var target string
	for _, t := range targets {
		if _, err := os.Stat(t); err == nil {
			target = t
			break
		}
	}
	if target == "" {
		return fmt.Errorf("%w: %s", domain.ErrNotFound, id)
	}

	if err := os.RemoveAll(target); err != nil {
		return err
	}
	// 追跡されていた場合は git add -A（パス無し）で削除をステージする。
	// パス指定の git add は、非追跡ファイル（adhocタスクなど）に対して
	// "pathspec did not match any files" で失敗するため、全体指定にする。
	if isGitRepo(r.root) {
		if err := runGit(ctx, r.root, "add", "-A"); err != nil {
			return err
		}
	}
	return nil
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}
