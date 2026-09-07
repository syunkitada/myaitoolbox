package entrypoint

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
)

type cliEnv struct {
	projName   string
	projPath   string
	configPath string
}

func newCLIEnv(t *testing.T) *cliEnv {
	t.Helper()
	e := &cliEnv{projName: "test", projPath: t.TempDir()}
	e.configPath = filepath.Join(t.TempDir(), "config.yaml")
	t.Setenv("MYBOX_CONFIG", e.configPath)
	return e
}

func (e *cliEnv) addProject(t *testing.T) {
	t.Helper()
	out := runCLIOk(t, "project", "add", e.projPath)
	fields := strings.Fields(out)
	require.Len(t, fields, 4, "expected `added project <name> (<path>)`")
	e.projName = fields[2]
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := NewRootCommand()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return buf.String(), err
}

func runCLIOk(t *testing.T, args ...string) string {
	t.Helper()
	out, err := runCLI(t, args...)
	require.NoError(t, err)
	return out
}

func setupCLIProject(t *testing.T) *cliEnv {
	t.Helper()
	e := newCLIEnv(t)
	e.addProject(t)
	return e
}

func TestCLIVersion(t *testing.T) {
	out := runCLIOk(t, "version")
	assert.Contains(t, out, "mybox")
}

func TestCLIProjectListAddRemove(t *testing.T) {
	e := newCLIEnv(t)

	out := runCLIOk(t, "project", "list")
	assert.NotContains(t, out, e.projName)

	e.addProject(t)
	out = runCLIOk(t, "project", "list")
	assert.Contains(t, out, e.projName)

	runCLIOk(t, "project", "remove", e.projName)
	out = runCLIOk(t, "project", "list")
	assert.NotContains(t, out, e.projName)
}

func TestCLIUnknownProject(t *testing.T) {
	e := newCLIEnv(t)
	e.addProject(t)
	_, err := runCLI(t, "--project", "nope", "task", "list")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCLITaskCreateJSONSchema(t *testing.T) {
	setupCLIProject(t)
	out := runCLIOk(t, "task", "create", "--json", "--name", "Build the thing")
	var task domain.Task
	require.NoError(t, json.Unmarshal([]byte(out), &task))
	assert.NotEmpty(t, task.ID)
	assert.Equal(t, "Build the thing", task.Title)
	assert.Equal(t, domain.TaskStatusTodo, task.Status)
	assert.Contains(t, task.ID, "build-the-thing")
}

func TestCLITaskLifecycle(t *testing.T) {
	e := setupCLIProject(t)
	id := runCLIOk(t, "task", "create", "--name", "Lifecycle task")
	id = trimOutput(id)

	out := runCLIOk(t, "task", "list", "--json")
	var tasks []domain.Task
	require.NoError(t, json.Unmarshal([]byte(out), &tasks))
	require.Len(t, tasks, 1)
	assert.Equal(t, id, tasks[0].ID)

	runCLIOk(t, "task", "set", "--status", "doing", "--priority", "high", "--assignee", "me", id)
	out = runCLIOk(t, "task", "show", id)
	assert.Contains(t, out, "doing")

	raw, err := os.ReadFile(filepath.Join(e.projPath, "tasks", id, "task.md"))
	require.NoError(t, err)
	content := string(raw)
	assert.Contains(t, content, "status: doing")
	assert.Contains(t, content, "priority: high")

	runCLIOk(t, "task", "archive", id)
	out = runCLIOk(t, "task", "list", "--json")
	require.NoError(t, json.Unmarshal([]byte(out), &tasks))
	assert.Len(t, tasks, 0)

	out = runCLIOk(t, "task", "list", "--all", "--json")
	require.NoError(t, json.Unmarshal([]byte(out), &tasks))
	require.Len(t, tasks, 1)
	assert.True(t, tasks[0].Archived)
}

func TestCLIFilesLifecycle(t *testing.T) {
	e := setupCLIProject(t)
	out := runCLIOk(t, "files", "create", "--json", "notes/alpha.md")
	var created map[string]string
	require.NoError(t, json.Unmarshal([]byte(out), &created))
	assert.Equal(t, "notes/alpha.md", created["path"])

	runCLIOk(t, "files", "mkdir", "docs")
	runCLIOk(t, "files", "create", "docs/README.md")

	out = runCLIOk(t, "files", "list", "--json")
	var entries []domain.FileEntry
	require.NoError(t, json.Unmarshal([]byte(out), &entries))
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	assert.Contains(t, paths, "notes/alpha.md")
	assert.Contains(t, paths, "docs")
	assert.Contains(t, paths, "docs/README.md")

	out = runCLIOk(t, "files", "list", "docs")
	assert.Contains(t, out, "docs/README.md")
	assert.NotContains(t, out, "notes/alpha.md")

	out = runCLIOk(t, "files", "show", "docs/README.md")
	assert.Equal(t, "", strings.TrimSpace(out))

	runCLIOk(t, "files", "move", "notes/alpha.md", "notes/beta.md")
	_, err := os.Stat(filepath.Join(e.projPath, "notes", "beta.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(e.projPath, "notes", "alpha.md"))
	require.Error(t, err)

	runCLIOk(t, "files", "copy", "notes/beta.md", "notes/beta-copy.md")
	_, err = os.Stat(filepath.Join(e.projPath, "notes", "beta-copy.md"))
	require.NoError(t, err)

	runCLIOk(t, "files", "rename", "notes/beta.md", "gamma.md")
	_, err = os.Stat(filepath.Join(e.projPath, "notes", "gamma.md"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(e.projPath, "notes", "beta.md"))
	require.Error(t, err)

	runCLIOk(t, "files", "delete", "notes/gamma.md")
	_, err = os.Stat(filepath.Join(e.projPath, "notes", "gamma.md"))
	require.Error(t, err)
}

func trimOutput(s string) string {
	return strings.TrimSpace(s)
}
