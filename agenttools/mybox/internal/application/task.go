package application

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type TaskUseCase struct {
	Tasks       domain.TaskRepository
	Template    domain.TemplateRenderer
	Prompts     domain.PromptTemplateRepository
	State       *StateUseCase
	Project     string
	ProjectPath string
}

type TaskFilter struct {
	Status string
	Tag    string
}

type TaskInput struct {
	Name string
	// Content nil uses the rendered template; a non-nil value can explicitly
	// provide an empty task.md.
	Content *string
	Status  string
	Due     string
	Tags    []string
}

// TaskPatch preserves field presence so callers can distinguish an omitted
// value from an explicit empty value.
type TaskPatch struct {
	Name   *string
	Status *string
	Due    *string
	Tags   *[]string
}

func NewTaskUseCase(tasks domain.TaskRepository, template domain.TemplateRenderer, prompts domain.PromptTemplateRepository, project string, projectPath string) *TaskUseCase {
	return &TaskUseCase{
		Tasks:       tasks,
		Template:    template,
		Prompts:     prompts,
		Project:     project,
		ProjectPath: projectPath,
	}
}

// SetState connects task renames to the project-scoped favorites and recent
// file state. It is optional so the use case remains easy to unit test with a
// repository-only setup.
func (u *TaskUseCase) SetState(state *StateUseCase) {
	u.State = state
}
func (u *TaskUseCase) List(ctx context.Context, filter TaskFilter) ([]domain.Task, error) {
	tasks, err := u.Tasks.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []domain.Task
	for _, t := range tasks {
		if filter.Status != "" && string(t.Status) != filter.Status {
			continue
		}
		if filter.Tag != "" && !contains(t.Tags, filter.Tag) {
			continue
		}
		// リポジトリはプロジェクト名を持たないため UseCase 層で補完する
		if t.Project == "" {
			t.Project = u.Project
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if ri, rj := rankTaskStatus(out[i].Status), rankTaskStatus(out[j].Status); ri != rj {
			return ri < rj
		}
		return out[i].Created.After(out[j].Created)
	})
	return out, nil
}

// taskStatusRank orders statuses by the workflow order shown on the GTD board
// (todo -> doing -> blocked -> review -> done) rather than lexicographically.
var taskStatusRank = map[domain.TaskStatus]int{
	domain.TaskStatusTodo:    0,
	domain.TaskStatusDoing:   1,
	domain.TaskStatusBlocked: 2,
	domain.TaskStatusReview:  3,
	domain.TaskStatusDone:    4,
}

func rankTaskStatus(s domain.TaskStatus) int {
	if r, ok := taskStatusRank[s]; ok {
		return r
	}
	return len(taskStatusRank)
}

func (u *TaskUseCase) Show(ctx context.Context, id string) (*domain.Task, error) {
	return u.Tasks.Find(ctx, id)
}

func (u *TaskUseCase) RenderTaskTemplate(name string) (string, error) {
	return u.Template.RenderTask(domain.TaskTemplateData{Name: strings.TrimSpace(name)})
}

func (u *TaskUseCase) Create(ctx context.Context, input TaskInput) (*domain.Task, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrInvalidArgument)
	}
	slug := slugify(name)
	if slug == "" {
		return nil, fmt.Errorf("%w: name must contain at least one letter or digit", domain.ErrInvalidArgument)
	}
	status, err := parseStatus(input.Status)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	task := &domain.Task{
		Status:  status,
		Due:     input.Due,
		Tags:    input.Tags,
		Project: u.Project,
		Created: now,
	}
	task.ID = now.Format("20060102") + "_" + slug
	if err := validateTaskDirectoryName(task.ID); err != nil {
		return nil, err
	}
	task.Title = task.ID
	content := ""
	if input.Content != nil {
		content = *input.Content
	} else {
		content, err = u.RenderTaskTemplate(name)
		if err != nil {
			return nil, err
		}
	}
	if err := u.Tasks.Create(ctx, task.ID, content); err != nil {
		return nil, err
	}
	if err := u.Tasks.Update(ctx, *task); err != nil {
		return nil, err
	}
	return task, nil
}

// CreateFromContent creates an execution task from an already rendered
// Markdown template. The template's body and unknown frontmatter fields are
// preserved, while runtime-owned status and automation fields are applied.
func (u *TaskUseCase) CreateFromContent(ctx context.Context, id, content string, tags []string) (*domain.Task, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: task id is required", domain.ErrInvalidArgument)
	}
	if err := validateTaskDirectoryName(id); err != nil {
		return nil, err
	}
	if err := u.Tasks.Create(ctx, id, content); err != nil {
		return nil, err
	}
	task, err := u.Tasks.Find(ctx, id)
	if err != nil {
		_ = u.Tasks.Delete(ctx, id)
		return nil, err
	}
	task.Project = u.Project
	task.Status = domain.TaskStatusTodo
	if tags != nil {
		task.Tags = append([]string(nil), tags...)
	}
	if err := u.Tasks.Update(ctx, *task); err != nil {
		_ = u.Tasks.Delete(ctx, id)
		return nil, err
	}
	return task, nil
}

func (u *TaskUseCase) Update(ctx context.Context, id string, input TaskInput) (*domain.Task, error) {
	task, err := u.Tasks.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	originalID := task.ID
	if input.Status != "" {
		status, parseErr := parseStatus(input.Status)
		if parseErr != nil {
			return nil, parseErr
		}
		task.Status = status
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		if err := u.renameTask(ctx, task, name); err != nil {
			return nil, err
		}
	}
	if input.Due != "" {
		task.Due = input.Due
	}
	if len(input.Tags) > 0 {
		task.Tags = input.Tags
	}
	if err := u.persistTask(ctx, task, originalID); err != nil {
		return nil, err
	}
	return task, nil
}

func (u *TaskUseCase) Patch(ctx context.Context, id string, patch TaskPatch) (*domain.Task, error) {
	task, err := u.Tasks.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	originalID := task.ID
	if patch.Status != nil {
		if *patch.Status == "" {
			return nil, fmt.Errorf("%w: invalid status %q", domain.ErrInvalidArgument, *patch.Status)
		}
		status, parseErr := parseStatus(*patch.Status)
		if parseErr != nil {
			return nil, parseErr
		}
		task.Status = status
	}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: name is required", domain.ErrInvalidArgument)
		}
		if err := u.renameTask(ctx, task, name); err != nil {
			return nil, err
		}
	}
	if patch.Due != nil {
		task.Due = *patch.Due
	}
	if patch.Tags != nil {
		task.Tags = append([]string(nil), (*patch.Tags)...)
	}
	if err := u.persistTask(ctx, task, originalID); err != nil {
		return nil, err
	}
	return task, nil
}

func (u *TaskUseCase) persistTask(ctx context.Context, task *domain.Task, originalID string) error {
	if err := u.Tasks.Update(ctx, *task); err != nil {
		if task.ID == originalID {
			return err
		}
		rollbackErr := u.Tasks.Rename(ctx, task.ID, originalID)
		stateRollbackErr := u.moveTaskPath(ctx, task.ID, originalID, task.Archived)
		task.ID = originalID
		task.Title = originalID
		if rollbackErr != nil || stateRollbackErr != nil {
			if rollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("rollback task rename: %w", rollbackErr))
			}
			if stateRollbackErr != nil {
				err = errors.Join(err, fmt.Errorf("rollback task path state: %w", stateRollbackErr))
			}
		}
		return err
	}
	return nil
}

func (u *TaskUseCase) Archive(ctx context.Context, id string) error {
	return u.Tasks.Archive(ctx, id)
}

func (u *TaskUseCase) Delete(ctx context.Context, id string) error {
	return u.Tasks.Delete(ctx, id)
}

func (u *TaskUseCase) renameTask(ctx context.Context, task *domain.Task, name string) error {
	originalID := task.ID
	newID, err := taskIDForName(task.ID, name)
	if err != nil {
		return err
	}
	if err := u.Tasks.Rename(ctx, originalID, newID); err != nil {
		return err
	}
	if err := u.moveTaskPath(ctx, originalID, newID, task.Archived); err != nil {
		rollbackErr := u.Tasks.Rename(ctx, newID, originalID)
		if rollbackErr != nil {
			return errors.Join(err, fmt.Errorf("rollback task rename: %w", rollbackErr))
		}
		return err
	}
	task.ID = newID
	task.Title = newID
	return nil
}

func (u *TaskUseCase) moveTaskPath(ctx context.Context, oldID, newID string, archived bool) error {
	if u.State == nil || oldID == newID {
		return nil
	}
	base := "_tasks"
	if archived {
		base = "_archives/tasks"
	}
	return u.State.MovePath(ctx, base+"/"+oldID, base+"/"+newID, u.Project)
}

func taskIDForName(oldID, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == oldID {
		if err := validateTaskDirectoryName(oldID); err != nil {
			return "", err
		}
		return oldID, nil
	}
	if len(name) >= 9 && name[8] == '_' && allASCIIDigits(name[:8]) {
		name = name[9:]
	}
	slug := slugify(name)
	if slug == "" {
		return "", fmt.Errorf("%w: name must contain at least one letter or digit", domain.ErrInvalidArgument)
	}
	if len(oldID) >= 9 && oldID[8] == '_' && allASCIIDigits(oldID[:8]) {
		id := oldID[:9] + slug
		return id, validateTaskDirectoryName(id)
	}
	return slug, validateTaskDirectoryName(slug)
}

func validateTaskDirectoryName(id string) error {
	if utf8.RuneCountInString(id) > domain.MaxTaskDirectoryNameLength {
		return fmt.Errorf("%w: task directory name must be at most %d characters", domain.ErrInvalidArgument, domain.MaxTaskDirectoryNameLength)
	}
	return nil
}

func allASCIIDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// FilePath resolves the absolute path of a task's markdown file.
func (u *TaskUseCase) FilePath(ctx context.Context, id string) (string, error) {
	task, err := u.Tasks.Find(ctx, id)
	if err != nil {
		return "", err
	}
	return u.FilePathFor(task), nil
}

// FilePathFor returns the absolute path of the task's markdown file.
func (u *TaskUseCase) FilePathFor(task *domain.Task) string {
	return filepath.Join(u.ProjectPath, u.RelativePathFor(task))
}

// RelativePathFor returns the task's markdown file path relative to the project
// root (always slash-separated), e.g. _tasks/20260101_xxx/task.md. This is the
// path used by herdr file agents (which start in the project directory).
func (u *TaskUseCase) RelativePathFor(task *domain.Task) string {
	return "_tasks/" + task.ID + "/task.md"
}

// RenderPrompt resolves the prompt for a task. When raw starts with '@' it is
// strictly a prompt template name (stored under prompts/); otherwise, when raw
// matches an existing prompt template name the template is used, and any other
// value is used verbatim as an inline prompt. The variable $task_file_path is
// expanded to the task's markdown file (relative to the project root).
func (u *TaskUseCase) RenderPrompt(ctx context.Context, raw string, task *domain.Task) (string, error) {
	return u.RenderPromptWithVars(ctx, raw, task, nil)
}

// RenderPromptWithVars is RenderPrompt with additional event variables used by
// automation triggers. Existing prompt behavior remains unchanged.
func (u *TaskUseCase) RenderPromptWithVars(ctx context.Context, raw string, task *domain.Task, extra map[string]string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	vars := map[string]string{
		"task_file_path": u.RelativePathFor(task),
	}
	for key, value := range extra {
		vars[key] = value
	}
	if u.Prompts == nil {
		return domain.ExpandPromptVariables(raw, vars), nil
	}
	forced := strings.HasPrefix(raw, "@")
	name := strings.TrimPrefix(raw, "@")
	if strings.TrimSpace(name) == "" {
		return "", nil
	}
	if forced || isBarePromptName(name) {
		rendered, err := u.Prompts.Render(ctx, name, vars)
		if err == nil {
			return rendered, nil
		}
		if forced || !errors.Is(err, domain.ErrNotFound) {
			return "", err
		}
	}
	return domain.ExpandPromptVariables(raw, vars), nil
}

// isBarePromptName reports whether name can refer to a prompt template file
// (a flat prompts/<name>.md, no path separators or whitespace).
func isBarePromptName(name string) bool {
	return name != "" &&
		!strings.ContainsAny(name, `/\`) &&
		!strings.Contains(name, "..") &&
		!strings.ContainsAny(name, " \t\n")
}

func parseStatus(s string) (domain.TaskStatus, error) {
	if s == "" {
		return domain.TaskStatusTodo, nil
	}
	switch domain.TaskStatus(s) {
	case domain.TaskStatusTodo, domain.TaskStatusDoing,
		domain.TaskStatusBlocked, domain.TaskStatusReview, domain.TaskStatusDone:
		return domain.TaskStatus(s), nil
	}
	return "", fmt.Errorf("%w: invalid status %q", domain.ErrInvalidArgument, s)
}

func slugify(s string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
