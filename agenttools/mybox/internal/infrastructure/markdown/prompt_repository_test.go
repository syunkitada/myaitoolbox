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

func TestPromptRepositoryProjectOverride(t *testing.T) {
	root := t.TempDir()
	repo := NewPromptRepository(root, "")

	// Project prompts/ takes precedence and supports project-relative vars.
	require.NoError(t, os.MkdirAll(filepath.Join(root, "prompts"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(root, "prompts", "do-the-task.md"),
		[]byte("project prompt for `$task_file_path`"),
		0o644,
	))

	out, err := repo.Render(context.Background(), "do-the-task", map[string]string{"task_file_path": "tasks/abc/task.md"})
	require.NoError(t, err)
	assert.Equal(t, "project prompt for `tasks/abc/task.md`", out)
}

func TestPromptRepositoryFallbackToDefaultProject(t *testing.T) {
	def := t.TempDir()
	root := t.TempDir()
	repo := NewPromptRepository(root, def)

	require.NoError(t, os.MkdirAll(filepath.Join(def, "prompts"), 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(def, "prompts", "review.md"),
		[]byte("from default: `$task_file_path`"),
		0o644,
	))

	out, err := repo.Render(context.Background(), "review", map[string]string{"task_file_path": "tasks/x/task.md"})
	require.NoError(t, err)
	assert.Equal(t, "from default: `tasks/x/task.md`", out)
}

func TestPromptRepositoryBuiltinFallback(t *testing.T) {
	repo := NewPromptRepository(t.TempDir(), "")

	out, err := repo.Render(context.Background(), "do-the-task", map[string]string{"task_file_path": "tasks/x/task.md"})
	require.NoError(t, err)
	assert.Contains(t, out, "`tasks/x/task.md` を実施してください")

	out, err = repo.Render(context.Background(), "plan-the-task", map[string]string{"task_file_path": "tasks/x/task.md"})
	require.NoError(t, err)
	assert.Contains(t, out, "`tasks/x/task.md` は未完成のドラフトです")
}

func TestPromptRepositoryNotFound(t *testing.T) {
	repo := NewPromptRepository(t.TempDir(), t.TempDir())

	_, err := repo.Render(context.Background(), "no-such-template", map[string]string{})
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestPromptRepositoryVariableEscaping(t *testing.T) {
	cases := []struct {
		in   string
		vars map[string]string
		want string
	}{
		{in: "$$literal", vars: nil, want: "$literal"},
		{in: "a $used b", vars: map[string]string{"used": "X"}, want: "a X b"},
		{in: "a $unknown b", vars: map[string]string{}, want: "a $unknown b"},
		{in: "a $no_var", vars: map[string]string{"other": "Y"}, want: "a $no_var"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, expandPromptVars(c.in, c.vars))
	}
}

func TestPromptRepositoryInvalidName(t *testing.T) {
	repo := NewPromptRepository(t.TempDir(), "")

	for _, name := range []string{"", "a/b", `a\b`, "..", "a..b"} {
		_, err := repo.Render(context.Background(), name, nil)
		assert.ErrorIs(t, err, domain.ErrInvalidArgument, "name=%q", name)
	}
}
