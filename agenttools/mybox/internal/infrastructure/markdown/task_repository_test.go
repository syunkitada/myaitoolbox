package markdown

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func TestTaskRepositoryLifecycle(t *testing.T) {
	root := t.TempDir()
	repo := NewTaskRepository(root)
	ctx := context.Background()

	content := "---\nid: 20260801_0900_fix-login\ntitle: fix-login\nstatus: todo\npriority: medium\ntags: []\n---\n\nbody"
	err := repo.Create(ctx, "20260801_0900_fix-login", content)
	require.NoError(t, err)

	list, err := repo.List(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "20260801_0900_fix-login", list[0].ID)
	assert.Equal(t, "fix-login", list[0].Title)
	assert.Equal(t, domain.TaskStatusTodo, list[0].Status)

	task, err := repo.Find(ctx, "20260801_0900_fix-login")
	require.NoError(t, err)
	assert.Equal(t, "fix-login", task.Title)

	task.Status = domain.TaskStatusDoing
	task.Priority = domain.TaskPriorityHigh
	err = repo.Update(ctx, *task)
	require.NoError(t, err)

	got, err := repo.Find(ctx, "20260801_0900_fix-login")
	require.NoError(t, err)
	assert.Equal(t, domain.TaskStatusDoing, got.Status)
	assert.Equal(t, domain.TaskPriorityHigh, got.Priority)
	assert.Equal(t, "body", got.Body)

	err = repo.Archive(ctx, "20260801_0900_fix-login")
	require.NoError(t, err)

	active, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, active)

	archived, err := repo.ListArchived(ctx)
	require.NoError(t, err)
	require.Len(t, archived, 1)
	assert.True(t, archived[0].Archived)

	found, err := repo.Find(ctx, "20260801_0900_fix-login")
	require.NoError(t, err)
	assert.True(t, found.Archived)
}

func TestTaskRepositoryFindMissing(t *testing.T) {
	repo := NewTaskRepository(t.TempDir())
	_, err := repo.Find(context.Background(), "missing")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTaskRepositoryCreateDuplicate(t *testing.T) {
	repo := NewTaskRepository(t.TempDir())
	ctx := context.Background()
	require.NoError(t, repo.Create(ctx, "task1", "content"))
	err := repo.Create(ctx, "task1", "content")
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestTaskRepositoryRejectInvalidID(t *testing.T) {
	repo := NewTaskRepository(t.TempDir())
	_, err := repo.Find(context.Background(), "../escape")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
	err = repo.Create(context.Background(), "a/b", "content")
	assert.ErrorIs(t, err, domain.ErrInvalidPath)
}

func TestTaskRepositoryListWithoutTasksDir(t *testing.T) {
	repo := NewTaskRepository(t.TempDir())
	list, err := repo.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestTaskRepositoryPreservesCustomFrontMatter(t *testing.T) {
	root := t.TempDir()
	repo := NewTaskRepository(root)
	ctx := context.Background()
	content := "---\nid: task1\ntitle: t1\nstatus: todo\npriority: medium\ncustom: keep-me\n---\n\nbody"
	require.NoError(t, repo.Create(ctx, "task1", content))

	task, err := repo.Find(ctx, "task1")
	require.NoError(t, err)
	task.Status = domain.TaskStatusDone
	require.NoError(t, repo.Update(ctx, *task))

	data, err := os.ReadFile(filepath.Join(root, "tasks", "task1", "task.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "custom: keep-me")
	assert.Contains(t, string(data), "status: done")
}
