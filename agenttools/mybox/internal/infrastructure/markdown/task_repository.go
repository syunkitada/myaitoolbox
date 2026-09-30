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
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/fsutil"
)

type TaskRepository struct {
	root string
}

func NewTaskRepository(root string) *TaskRepository {
	return &TaskRepository{root: root}
}

type taskFields struct {
	Title         string   `yaml:"title"`
	Description   string   `yaml:"description"`
	AgentKind     string   `yaml:"agent_kind"`
	Status        string   `yaml:"status"`
	Priority      string   `yaml:"priority"`
	Assignee      string   `yaml:"assignee"`
	Due           string   `yaml:"due"`
	PendingUntil  string   `yaml:"pending_until"`
	PendingReason string   `yaml:"pending_reason"`
	Tags          []string `yaml:"tags"`
}

func (r *TaskRepository) List(ctx context.Context) ([]domain.Task, error) {
	return r.list(filepath.Join(r.root, "_tasks"), false)
}

func (r *TaskRepository) ListArchived(ctx context.Context) ([]domain.Task, error) {
	return r.list(filepath.Join(r.root, "_archives", "tasks"), true)
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
	return r.readTaskFile(filepath.Join(dir, "task.md"), id, archived)
}

func (r *TaskRepository) readTaskFile(path string, id string, archived bool) (*domain.Task, error) {
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
		Description:   f.Description,
		AgentKind:     f.AgentKind,
		Status:        domain.TaskStatus(f.Status),
		Priority:      domain.TaskPriority(f.Priority),
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
	active := filepath.Join(r.root, "_tasks", id, "task.md")
	if _, err := os.Stat(active); err == nil {
		return r.readTask(filepath.Join(r.root, "_tasks", id), id, false)
	}
	archived := filepath.Join(r.root, "_archives", "tasks", id, "task.md")
	if _, err := os.Stat(archived); err == nil {
		return r.readTask(filepath.Join(r.root, "_archives", "tasks", id), id, true)
	}
	return nil, fmt.Errorf("%w: task %s", domain.ErrNotFound, id)
}

func (r *TaskRepository) Create(ctx context.Context, id string, content string) error {
	if err := validateTaskID(id); err != nil {
		return err
	}
	dir := filepath.Join(r.root, "_tasks", id)
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("%w: %s", domain.ErrAlreadyExists, id)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(filepath.Join(dir, "task.md"), []byte(content), 0o644)
}

func (r *TaskRepository) Update(ctx context.Context, task domain.Task) error {
	if err := validateTaskID(task.ID); err != nil {
		return err
	}
	path := filepath.Join(r.root, "_tasks", task.ID, "task.md")
	if task.Archived {
		path = filepath.Join(r.root, "_archives", "tasks", task.ID, "task.md")
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
	delete(fm, "type")
	delete(fm, "task_kind")
	fm["title"] = task.Title
	fm["description"] = task.Description
	fm["agent_kind"] = task.AgentKind
	fm["status"] = string(task.Status)
	fm["priority"] = string(task.Priority)
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
	return fsutil.WriteFileAtomic(path, []byte(out), 0o644)
}

func (r *TaskRepository) Archive(ctx context.Context, id string) error {
	if err := validateTaskID(id); err != nil {
		return err
	}
	src := filepath.Join(r.root, "_tasks", id)
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s", domain.ErrNotFound, id)
		} else {
			return err
		}
	}
	// ファイルエージェントがタスクディレクトリ内に作る一時作業ディレクトリ
	// （tmp/）はアーカイブ前に削除する。
	if err := removeTaskTmpDir(src); err != nil {
		return err
	}
	dst := filepath.Join(r.root, "_archives", "tasks", id)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err != nil {
		return err
	}
	// git mv は非追跡ファイル（未コミットの変更）を含むディレクトリで
	// "source directory is empty" 等で失敗するため、os.Rename + 対象パスだけの
	// git add -A で、追跡・非追跡どちらでも移動を正しくステージする。
	if isGitRepo(r.root) {
		if err := stageGitPaths(ctx, r.root, dst); err != nil {
			return err
		}
		return stageDeletedGitPaths(ctx, r.root, src)
	}
	return nil
}

// removeTaskTmpDir deletes the temporary working directory (tmp/) that file
// agents create inside a task directory. An absent tmp entry is skipped.
func removeTaskTmpDir(taskDir string) error {
	tmpDir := filepath.Join(taskDir, "tmp")
	info, err := os.Stat(tmpDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return os.RemoveAll(tmpDir)
}

func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	if err := validateTaskID(id); err != nil {
		return err
	}
	regular := filepath.Join(r.root, "_tasks", id)
	archivedRegular := filepath.Join(r.root, "_archives", "tasks", id)

	targets := []string{regular, archivedRegular}
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
	// 追跡されていた場合は対象パスだけをステージする。リポジトリ全体を
	// git add -A すると、利用者の無関係な変更までステージしてしまう。
	if isGitRepo(r.root) {
		if err := stageDeletedGitPaths(ctx, r.root, target); err != nil {
			return err
		}
	}
	return nil
}

func stageGitPaths(ctx context.Context, root string, paths ...string) error {
	relPaths, err := relativeGitPaths(root, paths...)
	if err != nil {
		return err
	}
	args := []string{"add", "-A", "--"}
	args = append(args, relPaths...)
	return runGit(ctx, root, args...)
}

func stageDeletedGitPaths(ctx context.Context, root string, paths ...string) error {
	relPaths, err := relativeGitPaths(root, paths...)
	if err != nil {
		return err
	}
	tracked := make([]string, 0, len(relPaths))
	for _, path := range relPaths {
		cmd := exec.CommandContext(ctx, "git", "ls-files", "--error-unmatch", "--", path)
		cmd.Dir = root
		if err := cmd.Run(); err == nil {
			tracked = append(tracked, path)
		} else {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if _, ok := err.(*exec.ExitError); !ok {
				return fmt.Errorf("git ls-files %s: %w", path, err)
			}
		}
	}
	if len(tracked) == 0 {
		return nil
	}
	args := []string{"add", "-u", "--"}
	args = append(args, tracked...)
	return runGit(ctx, root, args...)
}

func relativeGitPaths(root string, paths ...string) ([]string, error) {
	relPaths := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
			return nil, fmt.Errorf("%w: git path %s", domain.ErrInvalidPath, path)
		}
		relPaths = append(relPaths, filepath.ToSlash(rel))
	}
	return relPaths, nil
}

func validateTaskID(id string) error {
	if id == "" || id == "." || id == ".." || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, id)
	}
	return nil
}

func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--is-inside-work-tree")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
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
