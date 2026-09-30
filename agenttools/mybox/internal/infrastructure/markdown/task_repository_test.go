package markdown

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

func runGitForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	return string(out)
}

func initGitRepoForTest(t *testing.T, root string) {
	t.Helper()
	runGitForTest(t, root, "init")
	runGitForTest(t, root, "config", "user.email", "test@example.com")
	runGitForTest(t, root, "config", "user.name", "test")
}

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

func TestTaskRepositoryArchiveRemovesTmpDir(t *testing.T) {
	root := t.TempDir()
	repo := NewTaskRepository(root)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, "20260802_cleanup", "---\ntitle: cleanup\n---\n\n"))

	taskDir := filepath.Join(root, "_tasks", "20260802_cleanup")
	tmpDir := filepath.Join(taskDir, "tmp")
	require.NoError(t, os.MkdirAll(tmpDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "agent.log"), []byte("scratch"), 0o644))

	require.NoError(t, repo.Archive(ctx, "20260802_cleanup"))

	_, err := os.Stat(filepath.Join(root, "_tasks", "20260802_cleanup", "tmp"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(root, "_archives", "tasks", "20260802_cleanup", "tmp"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(root, "_archives", "tasks", "20260802_cleanup", "task.md"))
	require.NoError(t, err)
}

func TestTaskRepositoryArchiveWithoutTmpDir(t *testing.T) {
	root := t.TempDir()
	repo := NewTaskRepository(root)
	ctx := context.Background()

	require.NoError(t, repo.Create(ctx, "20260802_no-tmp", "---\ntitle: no tmp\n---\n\n"))

	require.NoError(t, repo.Archive(ctx, "20260802_no-tmp"))

	_, err := os.Stat(filepath.Join(root, "_archives", "tasks", "20260802_no-tmp", "task.md"))
	require.NoError(t, err)
}

func TestTaskRepositoryArchiveStagesOnlyTaskPaths(t *testing.T) {
	root := t.TempDir()
	initGitRepoForTest(t, root)
	repo := NewTaskRepository(root)

	const id = "20260802_scoped-archive"
	require.NoError(t, repo.Create(context.Background(), id, "---\ntitle: scoped archive\n---\n\n"))
	require.NoError(t, os.WriteFile(filepath.Join(root, "unrelated.md"), []byte("keep unstaged"), 0o644))

	require.NoError(t, repo.Archive(context.Background(), id))

	staged := runGitForTest(t, root, "diff", "--cached", "--name-status", "--no-renames")
	assert.Contains(t, staged, "_archives/tasks/"+id+"/task.md")
	assert.NotContains(t, staged, "unrelated.md")
	status := runGitForTest(t, root, "status", "--porcelain")
	assert.Contains(t, status, "?? unrelated.md")
}

func TestTaskRepositoryArchiveStagesTrackedTaskDeletion(t *testing.T) {
	root := t.TempDir()
	initGitRepoForTest(t, root)
	repo := NewTaskRepository(root)

	const id = "20260802_tracked-archive"
	require.NoError(t, repo.Create(context.Background(), id, "---\ntitle: tracked archive\n---\n\n"))
	runGitForTest(t, root, "add", "_tasks/"+id)
	runGitForTest(t, root, "commit", "-m", "add task")
	require.NoError(t, os.WriteFile(filepath.Join(root, "unrelated.md"), []byte("keep unstaged"), 0o644))

	require.NoError(t, repo.Archive(context.Background(), id))

	staged := runGitForTest(t, root, "diff", "--cached", "--name-status", "--no-renames")
	assert.Contains(t, staged, "_tasks/"+id+"/task.md")
	assert.Contains(t, staged, "_archives/tasks/"+id+"/task.md")
	assert.NotContains(t, staged, "unrelated.md")
	status := runGitForTest(t, root, "status", "--porcelain")
	assert.Contains(t, status, "?? unrelated.md")
}

func TestTaskRepositoryDeleteStagesOnlyTaskPaths(t *testing.T) {
	root := t.TempDir()
	initGitRepoForTest(t, root)
	repo := NewTaskRepository(root)

	const id = "20260802_scoped-delete"
	require.NoError(t, repo.Create(context.Background(), id, "---\ntitle: scoped delete\n---\n\n"))
	runGitForTest(t, root, "add", "_tasks/"+id)
	runGitForTest(t, root, "commit", "-m", "add task")
	require.NoError(t, os.WriteFile(filepath.Join(root, "unrelated.md"), []byte("keep unstaged"), 0o644))

	require.NoError(t, repo.Delete(context.Background(), id))

	staged := runGitForTest(t, root, "diff", "--cached", "--name-only")
	assert.Contains(t, staged, "_tasks/"+id+"/task.md")
	assert.NotContains(t, staged, "unrelated.md")
	status := runGitForTest(t, root, "status", "--porcelain")
	assert.Contains(t, status, "?? unrelated.md")
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

	data, err := os.ReadFile(filepath.Join(root, "_tasks", "task1", "task.md"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "custom: keep-me")
	assert.Contains(t, string(data), "status: done")
}

func TestTaskRepositoryReadsPendingFields(t *testing.T) {
	root := t.TempDir()
	repo := NewTaskRepository(root)
	ctx := context.Background()
	content := "---\nid: task1\ntitle: t1\nstatus: blocked\npriority: high\npending_until: 20260820\npending_reason: waiting for review\n---\n\nbody"
	require.NoError(t, repo.Create(ctx, "task1", content))

	task, err := repo.Find(ctx, "task1")
	require.NoError(t, err)
	assert.Equal(t, "20260820", task.PendingUntil)
	assert.Equal(t, "waiting for review", task.PendingReason)

	task.Status = domain.TaskStatusTodo
	require.NoError(t, repo.Update(ctx, *task))

	got, err := repo.Find(ctx, "task1")
	require.NoError(t, err)
	assert.Equal(t, "20260820", got.PendingUntil)
	assert.Equal(t, "waiting for review", got.PendingReason)
}

func TestTaskRepositoryIgnoresLegacyLayout(t *testing.T) {
	root := t.TempDir()
	repo := NewTaskRepository(root)
	ctx := context.Background()

	content := "---\ntitle: legacy task\nstatus: todo\npriority: high\n---\n\nbody"
	legacyActive := filepath.Join(root, "tasks", "20260902_legacy")
	require.NoError(t, os.MkdirAll(legacyActive, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyActive, "task.md"), []byte(content), 0o644))
	legacyArchived := filepath.Join(root, "archives", "tasks", "20260903_legacy")
	require.NoError(t, os.MkdirAll(legacyArchived, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(legacyArchived, "task.md"), []byte(content), 0o644))

	list, err := repo.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)

	archived, err := repo.ListArchived(ctx)
	require.NoError(t, err)
	assert.Empty(t, archived)

	_, err = repo.Find(ctx, "20260902_legacy")
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = repo.Find(ctx, "20260903_legacy")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTaskRepositoryLegacyMetadataIgnored(t *testing.T) {
	root := t.TempDir()
	repo := NewTaskRepository(root)
	ctx := context.Background()

	// task_kind / type のレガシーキーは type 区別としては扱われない。
	require.NoError(t, repo.Create(ctx, "20260904_legacy", "---\ntitle: legacy\ntype: adhoc\ntask_kind: adhoc\n---\n\n"))
	task, err := repo.Find(ctx, "20260904_legacy")
	require.NoError(t, err)
	assert.Equal(t, "legacy", task.Title)
	assert.Equal(t, domain.TaskStatusTodo, task.Status)

	// Update で task_kind / type キーはフロントマターから除去される。
	task.Status = domain.TaskStatusDone
	require.NoError(t, repo.Update(ctx, *task))
	data, err := os.ReadFile(filepath.Join(root, "_tasks", "20260904_legacy", "task.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "task_kind")
	assert.NotContains(t, string(data), "adhoc")
}
