package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type TaskUseCase struct {
	Tasks    domain.TaskRepository
	Template domain.TemplateRenderer
	Project  string
}

type TaskFilter struct {
	All      bool
	Status   string
	Tag      string
	Assignee string
	Type     string
}

type TaskInput struct {
	Name     string
	Status   string
	Priority string
	Type     string
	Assignee string
	Due      string
	Tags     []string
}

func NewTaskUseCase(tasks domain.TaskRepository, template domain.TemplateRenderer, project string) *TaskUseCase {
	return &TaskUseCase{Tasks: tasks, Template: template, Project: project}
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
		if filter.Type != "" && string(t.Type) != filter.Type {
			continue
		}
		// リポジトリはプロジェクト名を持たないため UseCase 層で補完する
		if t.Project == "" {
			t.Project = u.Project
		}
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Status != out[j].Status {
			return out[i].Status < out[j].Status
		}
		return out[i].Created.After(out[j].Created)
	})
	return out, nil
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
	taskType, err := parseType(input.Type)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	task := &domain.Task{
		Title:    name,
		Status:   status,
		Priority: priority,
		Type:     taskType,
		Assignee: input.Assignee,
		Due:      input.Due,
		Tags:     input.Tags,
		Project:  u.Project,
		Created:  now,
	}
	if taskType == domain.TaskTypeAdhoc {
		task.ID = now.Format("20060102") + "_" + slugify(name)
		content, err := u.Template.RenderAdhocTask(domain.TaskTemplateData{
			Name: task.Title,
		})
		if err != nil {
			return nil, err
		}
		if err := u.Tasks.CreateAdhoc(ctx, task.ID, content); err != nil {
			return nil, err
		}
	} else {
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
	}
	if len(task.Tags) > 0 {
		if err := u.Tasks.Update(ctx, *task); err != nil {
			return nil, err
		}
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
	if input.Type != "" {
		taskType, err := parseType(input.Type)
		if err != nil {
			return nil, err
		}
		task.Type = taskType
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

func (u *TaskUseCase) Archive(ctx context.Context, id string) error {
	return u.Tasks.Archive(ctx, id)
}

func (u *TaskUseCase) Delete(ctx context.Context, id string) error {
	return u.Tasks.Delete(ctx, id)
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

func parseType(s string) (domain.TaskType, error) {
	if s == "" {
		return domain.TaskTypeRegular, nil
	}
	switch domain.TaskType(s) {
	case domain.TaskTypeRegular, domain.TaskTypeAdhoc:
		return domain.TaskType(s), nil
	}
	return "", fmt.Errorf("%w: invalid type %q", domain.ErrInvalidArgument, s)
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
