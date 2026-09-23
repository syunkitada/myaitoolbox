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

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type TaskUseCase struct {
	Tasks       domain.TaskRepository
	Template    domain.TemplateRenderer
	Prompts     domain.PromptTemplateRepository
	Project     string
	ProjectPath string
}

type TaskFilter struct {
	All      bool
	Status   string
	Tag      string
	Assignee string
}

type TaskInput struct {
	Name        string
	Description string
	AgentKind   string
	Status      string
	Priority    string
	Assignee    string
	Due         string
	Tags        []string
}

// TaskPatch preserves field presence so callers can distinguish an omitted
// value from an explicit empty value.
type TaskPatch struct {
	Name        *string
	Description *string
	AgentKind   *string
	Status      *string
	Priority    *string
	Assignee    *string
	Due         *string
	Tags        *[]string
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
func (u *TaskUseCase) List(ctx context.Context, filter TaskFilter) ([]domain.Task, error) {
	tasks, err := u.Tasks.List(ctx)
	if err != nil {
		return nil, err
	}
	if filter.All {
		archived, err := u.Tasks.ListArchived(ctx)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, archived...)
	}
	var out []domain.Task
	for _, t := range tasks {
		if filter.Status != "" && string(t.Status) != filter.Status {
			continue
		}
		if filter.Tag != "" && !contains(t.Tags, filter.Tag) {
			continue
		}
		if filter.Assignee != "" && t.Assignee != filter.Assignee {
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

func (u *TaskUseCase) Create(ctx context.Context, input TaskInput) (*domain.Task, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, fmt.Errorf("%w: name is required", domain.ErrInvalidArgument)
	}
	status, err := parseStatus(input.Status)
	if err != nil {
		return nil, err
	}
	priority, err := parsePriority(input.Priority)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	task := &domain.Task{
		Title:       name,
		Description: strings.TrimSpace(input.Description),
		AgentKind:   strings.TrimSpace(input.AgentKind),
		Status:      status,
		Priority:    priority,
		Assignee:    input.Assignee,
		Due:         input.Due,
		Tags:        input.Tags,
		Project:     u.Project,
		Created:     now,
	}
	task.ID = now.Format("20060102") + "_" + slugify(name)
	content, err := u.Template.RenderTask(domain.TaskTemplateData{
		Name: task.Title,
	})
	if err != nil {
		return nil, err
	}
	if err := u.Tasks.Create(ctx, task.ID, content); err != nil {
		return nil, err
	}
	if err := u.Tasks.Update(ctx, *task); err != nil {
		return nil, err
	}
	return task, nil
}

func (u *TaskUseCase) Update(ctx context.Context, id string, input TaskInput) (*domain.Task, error) {
	task, err := u.Tasks.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if name := strings.TrimSpace(input.Name); name != "" {
		task.Title = name
	}
	if input.Description != "" {
		task.Description = strings.TrimSpace(input.Description)
	}
	if input.AgentKind != "" {
		task.AgentKind = strings.TrimSpace(input.AgentKind)
	}
	if input.Status != "" {
		status, err := parseStatus(input.Status)
		if err != nil {
			return nil, err
		}
		task.Status = status
	}
	if input.Priority != "" {
		priority, err := parsePriority(input.Priority)
		if err != nil {
			return nil, err
		}
		task.Priority = priority
	}
	if input.Assignee != "" {
		task.Assignee = input.Assignee
	}
	if input.Due != "" {
		task.Due = input.Due
	}
	if len(input.Tags) > 0 {
		task.Tags = input.Tags
	}
	if err := u.Tasks.Update(ctx, *task); err != nil {
		return nil, err
	}
	return task, nil
}

func (u *TaskUseCase) Patch(ctx context.Context, id string, patch TaskPatch) (*domain.Task, error) {
	task, err := u.Tasks.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return nil, fmt.Errorf("%w: name is required", domain.ErrInvalidArgument)
		}
		task.Title = name
	}
	if patch.Description != nil {
		task.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.AgentKind != nil {
		task.AgentKind = strings.TrimSpace(*patch.AgentKind)
	}
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
	if patch.Priority != nil {
		if *patch.Priority == "" {
			return nil, fmt.Errorf("%w: invalid priority %q", domain.ErrInvalidArgument, *patch.Priority)
		}
		priority, parseErr := parsePriority(*patch.Priority)
		if parseErr != nil {
			return nil, parseErr
		}
		task.Priority = priority
	}
	if patch.Assignee != nil {
		task.Assignee = *patch.Assignee
	}
	if patch.Due != nil {
		task.Due = *patch.Due
	}
	if patch.Tags != nil {
		task.Tags = append([]string(nil), (*patch.Tags)...)
	}
	if err := u.Tasks.Update(ctx, *task); err != nil {
		return nil, err
	}
	return task, nil
}

func (u *TaskUseCase) Archive(ctx context.Context, id string) error {
	return u.Tasks.Archive(ctx, id)
}

func (u *TaskUseCase) Delete(ctx context.Context, id string) error {
	return u.Tasks.Delete(ctx, id)
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
// root (always slash-separated), e.g. tasks/20260101_xxx/task.md. This is the
// path used by herdr file agents (which start in the project directory).
func (u *TaskUseCase) RelativePathFor(task *domain.Task) string {
	return "tasks/" + task.ID + "/task.md"
}

// RenderPrompt resolves the prompt for a task. When raw starts with '@' it is
// strictly a prompt template name (stored under prompts/); otherwise, when raw
// matches an existing prompt template name the template is used, and any other
// value is used verbatim as an inline prompt. The variable $task_file_path is
// expanded to the task's markdown file (relative to the project root).
func (u *TaskUseCase) RenderPrompt(ctx context.Context, raw string, task *domain.Task) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	vars := map[string]string{
		"task_file_path": u.RelativePathFor(task),
	}
	if u.Prompts == nil {
		return raw, nil
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
	return raw, nil
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

func parsePriority(s string) (domain.TaskPriority, error) {
	if s == "" {
		return domain.TaskPriorityMedium, nil
	}
	switch domain.TaskPriority(s) {
	case domain.TaskPriorityLow, domain.TaskPriorityMedium,
		domain.TaskPriorityHigh, domain.TaskPriorityUrgent:
		return domain.TaskPriority(s), nil
	}
	return "", fmt.Errorf("%w: invalid priority %q", domain.ErrInvalidArgument, s)
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
