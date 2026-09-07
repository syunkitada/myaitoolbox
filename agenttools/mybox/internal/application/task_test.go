package application

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/infrastructure/markdown"
)

func newTaskUC(t *testing.T) *TaskUseCase {
	t.Helper()
	root := t.TempDir()
	return NewTaskUseCase(
		markdown.NewTaskRepository(root),
		markdown.NewTemplateRenderer(root, ""),
		markdown.NewPromptRepository(root, ""),
		"test",
		root,
	)
}

func TestTaskCreateUpdateArchive(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	task, err := uc.Create(ctx, TaskInput{Name: "Fix Login Bug"})
	require.NoError(t, err)
	assert.Contains(t, task.ID, "fix-login-bug")
	assert.Equal(t, domain.TaskStatusTodo, task.Status)
	assert.Equal(t, "test", task.Project)

	updated, err := uc.Update(ctx, task.ID, TaskInput{Status: "doing", Priority: "high", Assignee: "owner"})
	require.NoError(t, err)
	assert.Equal(t, domain.TaskStatusDoing, updated.Status)
	assert.Equal(t, domain.TaskPriorityHigh, updated.Priority)
	assert.Equal(t, "owner", updated.Assignee)

	got, err := uc.Show(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TaskStatusDoing, got.Status)

	require.NoError(t, uc.Archive(ctx, task.ID))
	list, err := uc.List(ctx, TaskFilter{})
	require.NoError(t, err)
	assert.Empty(t, list)

	all, err := uc.List(ctx, TaskFilter{All: true})
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.True(t, all[0].Archived)
}

func TestTaskListFilter(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	t1, err := uc.Create(ctx, TaskInput{Name: "alpha", Tags: []string{"web"}})
	require.NoError(t, err)
	t2, err := uc.Create(ctx, TaskInput{Name: "beta", Tags: []string{"infra"}})
	require.NoError(t, err)
	_, _ = t1, t2

	list, err := uc.List(ctx, TaskFilter{Tag: "web"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "alpha", list[0].Title)

	list, err = uc.List(ctx, TaskFilter{Status: "todo"})
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestTaskCreateEmptyName(t *testing.T) {
	uc := newTaskUC(t)
	_, err := uc.Create(context.Background(), TaskInput{Name: "   "})
	assert.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestTaskUpdateInvalidStatus(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	task, err := uc.Create(ctx, TaskInput{Name: "x"})
	require.NoError(t, err)
	_, err = uc.Update(ctx, task.ID, TaskInput{Status: "bogus"})
	assert.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestTaskShowMissing(t *testing.T) {
	uc := newTaskUC(t)
	_, err := uc.Show(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTaskCreateAdhoc(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	task, err := uc.Create(ctx, TaskInput{Name: "Review PR #123", Type: "adhoc"})
	require.NoError(t, err)
	assert.Equal(t, domain.TaskTypeAdhoc, task.Type)
	assert.Contains(t, task.ID, "review-pr-123")

	got, err := uc.Show(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.TaskTypeAdhoc, got.Type)

	err = uc.Archive(ctx, task.ID)
	assert.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestTaskListFilterByType(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	_, err := uc.Create(ctx, TaskInput{Name: "regular task"})
	require.NoError(t, err)
	_, err = uc.Create(ctx, TaskInput{Name: "adhoc task", Type: "adhoc"})
	require.NoError(t, err)

	list, err := uc.List(ctx, TaskFilter{Type: "adhoc"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, domain.TaskTypeAdhoc, list[0].Type)

	list, err = uc.List(ctx, TaskFilter{Type: "regular"})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, domain.TaskTypeRegular, list[0].Type)
}

func TestTaskCreateInvalidType(t *testing.T) {
	uc := newTaskUC(t)
	_, err := uc.Create(context.Background(), TaskInput{Name: "x", Type: "bogus"})
	assert.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestTaskDescriptionAgentKind(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	task, err := uc.Create(ctx, TaskInput{Name: "Ship parser", Description: "parse everything", AgentKind: "opencode"})
	require.NoError(t, err)
	assert.Equal(t, "parse everything", task.Description)
	assert.Equal(t, "opencode", task.AgentKind)

	got, err := uc.Show(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "parse everything", got.Description)
	assert.Equal(t, "opencode", got.AgentKind)

	updated, err := uc.Update(ctx, task.ID, TaskInput{AgentKind: "codex"})
	require.NoError(t, err)
	assert.Equal(t, "parse everything", updated.Description)
	assert.Equal(t, "codex", updated.AgentKind)

	got, err = uc.Show(ctx, task.ID)
	require.NoError(t, err)
	assert.Equal(t, "codex", got.AgentKind)
}

func TestTaskAdhocFilePaths(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	regular, err := uc.Create(ctx, TaskInput{Name: "regular"})
	require.NoError(t, err)
	assert.Equal(t, "tasks/"+regular.ID+"/task.md", uc.RelativePathFor(regular))

	adhoc, err := uc.Create(ctx, TaskInput{Name: "adhoc", Type: "adhoc"})
	require.NoError(t, err)
	assert.Equal(t, "tasks/"+adhoc.ID+"/task.md", uc.RelativePathFor(adhoc))
}

func TestTaskRenderPrompt(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	task, err := uc.Create(ctx, TaskInput{Name: "implement login"})
	require.NoError(t, err)

	require.NoError(t, os.MkdirAll(filepath.Join(uc.ProjectPath, "prompts"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(uc.ProjectPath, "prompts", "do-the-task.md"),
		[]byte("`$task_file_path` を実施してください"),
		0o644,
	))

	// Template matched by bare name.
	rendered, err := uc.RenderPrompt(ctx, "do-the-task", task)
	require.NoError(t, err)
	assert.Equal(t, "`tasks/"+task.ID+"/task.md` を実施してください", rendered)

	// Explicit @ template reference.
	rendered, err = uc.RenderPrompt(ctx, "@do-the-task", task)
	require.NoError(t, err)
	assert.Equal(t, "`tasks/"+task.ID+"/task.md` を実施してください", rendered)

	// Inline prompt that does not match any template.
	rendered, err = uc.RenderPrompt(ctx, "DO IT NOW / do not $expand", task)
	require.NoError(t, err)
	assert.Equal(t, "DO IT NOW / do not $expand", rendered)

	// Forced template that is missing surfaces the error.
	_, err = uc.RenderPrompt(ctx, "@missing-template", task)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Empty prompt is a no-op.
	rendered, err = uc.RenderPrompt(ctx, "  ", task)
	require.NoError(t, err)
	assert.Equal(t, "", rendered)
}
