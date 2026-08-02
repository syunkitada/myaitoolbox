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

func TestCLITaskSearch(t *testing.T) {
	setupCLIProject(t)
	runCLIOk(t, "task", "create", "--name", "OAuth callback handler")
	out := runCLIOk(t, "task", "search", "oauth", "--json")
	var results []domain.SearchResult
	require.NoError(t, json.Unmarshal([]byte(out), &results))
	require.Len(t, results, 1)
	assert.Equal(t, domain.SearchTypeTask, results[0].Type)
	assert.Contains(t, results[0].Title, "OAuth")
}

func TestCLIKnowledgeLifecycle(t *testing.T) {
	e := setupCLIProject(t)
	path := runCLIOk(t, "knowledge", "create", "--json", "notes/alpha")
	var k domain.Knowledge
	require.NoError(t, json.Unmarshal([]byte(path), &k))
	assert.Equal(t, "notes/alpha", k.Path)

	out := runCLIOk(t, "knowledge", "list", "--json")
	var list []domain.Knowledge
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "notes/alpha", list[0].Path)

	out = runCLIOk(t, "knowledge", "show", "notes/alpha")
	assert.Contains(t, out, "notes/alpha")

	runCLIOk(t, "knowledge", "move", "notes/alpha", "notes/beta")
	_, err := os.Stat(filepath.Join(e.projPath, "knowledge", "notes", "beta.md"))
	require.NoError(t, err)

	runCLIOk(t, "knowledge", "rename", "notes/beta", "gamma")
	_, err = os.Stat(filepath.Join(e.projPath, "knowledge", "notes", "gamma.md"))
	require.NoError(t, err)
}

func TestCLISearchCrossType(t *testing.T) {
	e := setupCLIProject(t)
	runCLIOk(t, "task", "create", "--name", "OAuth login flow")
	runCLIOk(t, "knowledge", "create", "docs/oauth")
	require.NoError(t, os.WriteFile(
		filepath.Join(e.projPath, "knowledge", "docs", "oauth.md"),
		[]byte("# OAuth\n\nImplement the oauth flow.\n"), 0o644,
	))

	out := runCLIOk(t, "search", "oauth", "--json")
	var results []domain.SearchResult
	require.NoError(t, json.Unmarshal([]byte(out), &results))
	types := map[domain.SearchType]bool{}
	for _, r := range results {
		types[r.Type] = true
	}
	assert.True(t, types[domain.SearchTypeTask], "expected a task hit")
	assert.True(t, types[domain.SearchTypeKnowledge], "expected a knowledge hit")

	out = runCLIOk(t, "search", "oauth", "--type", "knowledge", "--json")
	require.NoError(t, json.Unmarshal([]byte(out), &results))
	assert.Len(t, results, 1)
	assert.Equal(t, domain.SearchTypeKnowledge, results[0].Type)

	_, err := runCLI(t, "search", "oauth", "--type", "bogus")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid --type")
}

func trimOutput(s string) string {
	return strings.TrimSpace(s)
}
