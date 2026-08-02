package application

import (
	"context"
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
		"test",
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
