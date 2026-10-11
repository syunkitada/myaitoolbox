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

	updated, err := uc.Update(ctx, task.ID, TaskInput{Status: "doing"})
	require.NoError(t, err)
	assert.Equal(t, domain.TaskStatusDoing, updated.Status)

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

func TestTaskCreateOmitsDeprecatedMetadata(t *testing.T) {
	uc := newTaskUC(t)

	task, err := uc.Create(context.Background(), TaskInput{Name: "minimal task"})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(uc.ProjectPath, "_tasks", task.ID, "task.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(content), "priority:")
	assert.NotContains(t, string(content), "agent_kind:")
	assert.NotContains(t, string(content), "assignee:")
	assert.NotContains(t, string(content), "description:")
}

func TestTaskUpdatePreservesDeprecatedMetadataInExistingFile(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	content := "---\ntitle: legacy task\nstatus: todo\npriority: high\nagent_kind: opencode\nassignee: owner\ndescription: legacy summary\n---\n\nbody"
	id := "20260927_legacy-task"
	taskDir := filepath.Join(uc.ProjectPath, "_tasks", id)
	require.NoError(t, os.MkdirAll(taskDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(taskDir, "task.md"), []byte(content), 0o644))

	_, err := uc.Update(ctx, id, TaskInput{Status: "doing"})
	require.NoError(t, err)

	updated, err := os.ReadFile(filepath.Join(uc.ProjectPath, "_tasks", id, "task.md"))
	require.NoError(t, err)
	assert.Contains(t, string(updated), "priority: high")
	assert.Contains(t, string(updated), "agent_kind: opencode")
	assert.Contains(t, string(updated), "assignee: owner")
	assert.Contains(t, string(updated), "description: legacy summary")
}

func TestTaskCreateFromContent(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	content := "---\ntitle: Daily report\nstatus: done\npriority: high\nagent_kind: opencode\nassignee: owner\ndescription: legacy summary\ncustom: preserve\n---\n\nPrepare the report."

	task, err := uc.CreateFromContent(ctx, "20260927_daily-report", content, []string{"scheduled"})
	require.NoError(t, err)
	assert.Equal(t, "20260927_daily-report", task.ID)
	assert.Equal(t, "Daily report", task.Title)
	assert.Equal(t, domain.TaskStatusTodo, task.Status)
	assert.Equal(t, []string{"scheduled"}, task.Tags)

	data, err := os.ReadFile(filepath.Join(uc.ProjectPath, "_tasks", task.ID, "task.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "priority:")
	assert.NotContains(t, string(data), "agent_kind:")
	assert.NotContains(t, string(data), "assignee:")
	assert.NotContains(t, string(data), "description:")
	assert.Contains(t, string(data), "custom: preserve")
	assert.Contains(t, string(data), "Prepare the report.")
}

func TestTaskCreateFromContentRejectsDuplicateName(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	_, err := uc.Create(ctx, TaskInput{Name: "Daily report"})
	require.NoError(t, err)

	content := "---\ntitle: Daily report\n---\n\nGenerated report."
	_, err = uc.CreateFromContent(ctx, "20260927_daily-report-2", content, nil)
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)

	_, err = uc.Show(ctx, "20260927_daily-report-2")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestTaskCreateUsesProvidedContent(t *testing.T) {
	uc := newTaskUC(t)
	content := "---\ntitle: Draft\npriority: high\nagent_kind: opencode\nassignee: owner\ndescription: legacy summary\n---\n\n## Custom body\n\n- supplied by the user\n"

	task, err := uc.Create(context.Background(), TaskInput{Name: "Custom task", Content: &content})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(uc.ProjectPath, "_tasks", task.ID, "task.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "priority:")
	assert.NotContains(t, string(data), "agent_kind:")
	assert.NotContains(t, string(data), "assignee:")
	assert.NotContains(t, string(data), "description:")
	assert.Contains(t, string(data), "## Custom body")
	assert.Contains(t, string(data), "- supplied by the user")
}

func TestTaskCreateStripsDeprecatedMetadataFromProjectTemplate(t *testing.T) {
	uc := newTaskUC(t)
	templateDir := filepath.Join(uc.ProjectPath, "templates", "task")
	require.NoError(t, os.MkdirAll(templateDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(templateDir, "task.md"), []byte("---\ntitle: {{.Name}}\npriority: high\nagent_kind: opencode\nassignee: owner\ndescription: legacy summary\n---\n\nProject template body\n"), 0o644))

	task, err := uc.Create(context.Background(), TaskInput{Name: "Project template task"})
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(uc.ProjectPath, "_tasks", task.ID, "task.md"))
	require.NoError(t, err)
	assert.NotContains(t, string(data), "priority:")
	assert.NotContains(t, string(data), "agent_kind:")
	assert.NotContains(t, string(data), "assignee:")
	assert.NotContains(t, string(data), "description:")
	assert.Contains(t, string(data), "Project template body")
}

func TestRenderTaskTemplate(t *testing.T) {
	uc := newTaskUC(t)

	content, err := uc.RenderTaskTemplate("Template task")
	require.NoError(t, err)
	assert.Contains(t, content, "title: Template task")
	assert.Contains(t, content, "## TODO")
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

func TestTaskCreateRejectsDuplicateName(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	content := "---\ntitle: Existing task\n---\n"
	require.NoError(t, uc.Tasks.Create(ctx, "20260101_existing", content))
	existing, err := uc.Tasks.Find(ctx, "20260101_existing")
	require.NoError(t, err)
	existing.Title = "日本語のタスク"
	require.NoError(t, uc.Tasks.Update(ctx, *existing))

	_, err = uc.Create(ctx, TaskInput{Name: " 日本語のタスク "})
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
}

func TestTaskUpdateRejectsDuplicateName(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	first, err := uc.Create(ctx, TaskInput{Name: "first"})
	require.NoError(t, err)
	second, err := uc.Create(ctx, TaskInput{Name: "second"})
	require.NoError(t, err)

	_, err = uc.Update(ctx, second.ID, TaskInput{Name: " first "})
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)

	got, err := uc.Show(ctx, second.ID)
	require.NoError(t, err)
	assert.Equal(t, "second", got.Title)
	assert.NotEqual(t, first.ID, second.ID)
}

func TestTaskPatchRejectsDuplicateNameIncludingArchivedTasks(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	archived, err := uc.Create(ctx, TaskInput{Name: "archived task"})
	require.NoError(t, err)
	require.NoError(t, uc.Archive(ctx, archived.ID))

	current, err := uc.Create(ctx, TaskInput{Name: "current task"})
	require.NoError(t, err)
	name := "archived task"
	_, err = uc.Patch(ctx, current.ID, TaskPatch{Name: &name})

	assert.ErrorIs(t, err, domain.ErrAlreadyExists)
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

func TestTaskCreateInvalidStatus(t *testing.T) {
	uc := newTaskUC(t)
	_, err := uc.Create(context.Background(), TaskInput{Name: "x", Status: "bogus"})
	assert.ErrorIs(t, err, domain.ErrInvalidArgument)
}

func TestTaskPatchClearsOptionalFields(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()
	task, err := uc.Create(ctx, TaskInput{
		Name: "clear fields",
		Due:  "2026-09-21",
		Tags: []string{"one", "two"},
	})
	require.NoError(t, err)
	empty := ""
	emptyTags := []string{}
	_, err = uc.Patch(ctx, task.ID, TaskPatch{
		Due:  &empty,
		Tags: &emptyTags,
	})
	require.NoError(t, err)
	got, err := uc.Show(ctx, task.ID)
	require.NoError(t, err)
	assert.Empty(t, got.Due)
	assert.Empty(t, got.Tags)
}

func TestTaskFilePaths(t *testing.T) {
	uc := newTaskUC(t)
	ctx := context.Background()

	task, err := uc.Create(ctx, TaskInput{Name: "some task"})
	require.NoError(t, err)
	assert.Equal(t, "_tasks/"+task.ID+"/task.md", uc.RelativePathFor(task))
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
	assert.Equal(t, "`_tasks/"+task.ID+"/task.md` を実施してください", rendered)

	// Explicit @ template reference.
	rendered, err = uc.RenderPrompt(ctx, "@do-the-task", task)
	require.NoError(t, err)
	assert.Equal(t, "`_tasks/"+task.ID+"/task.md` を実施してください", rendered)

	// Inline prompt that does not match any template.
	rendered, err = uc.RenderPrompt(ctx, "DO IT NOW / do not $expand", task)
	require.NoError(t, err)
	assert.Equal(t, "DO IT NOW / do not $expand", rendered)

	// Inline prompts also expand task variables.
	rendered, err = uc.RenderPrompt(ctx, "'$task_file_path' を実施してください。", task)
	require.NoError(t, err)
	assert.Equal(t, "'_tasks/"+task.ID+"/task.md' を実施してください。", rendered)

	// Forced template that is missing surfaces the error.
	_, err = uc.RenderPrompt(ctx, "@missing-template", task)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	// Empty prompt is a no-op.
	rendered, err = uc.RenderPrompt(ctx, "  ", task)
	require.NoError(t, err)
	assert.Equal(t, "", rendered)
}
