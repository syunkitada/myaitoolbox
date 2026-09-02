package entrypoint

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func herdrTestServer(t *testing.T, run herdrRunFunc) *Server {
	t.Helper()
	s, _ := newTestServer(t, false)
	s.herdrRun = run
	return s
}

const herdrWorkspacesJSON = `{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[` +
	`{"active_tab_id":"w5:t1","agent_status":"unknown","focused":false,"label":"home_ex","number":1,"pane_count":1,"tab_count":1,"workspace_id":"w5"},` +
	`{"active_tab_id":"w7:t1","agent_status":"working","focused":true,"label":"test","number":2,"pane_count":2,"tab_count":2,"workspace_id":"w7"}]}}`

const herdrAgentsJSON = `{"id":"cli:agent:list","result":{"agents":[` +
	`{"agent":"opencode","agent_status":"working","cwd":"/tmp/test","focused":true,"pane_id":"w7:p1","revision":6,"screen_detection_skipped":true,"state_change_seq":25,"tab_id":"w7:t1","terminal_id":"term_1","terminal_title":"OC | do stuff","terminal_title_stripped":"OC | do stuff","workspace_id":"w7"}]}}`

const herdrTabsJSON = `{"id":"cli:tab:list","result":{"tabs":[` +
	`{"agent_status":"unknown","focused":false,"label":"2","number":2,"pane_count":1,"tab_id":"w7:t2","workspace_id":"w7"},` +
	`{"agent_status":"working","focused":true,"label":"main","number":1,"pane_count":2,"tab_id":"w7:t1","workspace_id":"w7"}]}}`

const herdrPanesJSON = `{"id":"cli:pane:list","result":{"panes":[` +
	`{"agent_status":"unknown","cwd":"/tmp/test/sub","focused":false,"pane_id":"w7:p2","tab_id":"w7:t1","workspace_id":"w7","title":"review logs"},` +
	`{"agent_status":"working","cwd":"/tmp/test","focused":true,"pane_id":"w7:p1","tab_id":"w7:t1","workspace_id":"w7"}]}}`

func herdrStubRunner(calls *[][]string) herdrRunFunc {
	return func(ctx context.Context, args ...string) ([]byte, error) {
		*calls = append(*calls, args)
		switch {
		case args[0] == "workspace":
			return []byte(herdrWorkspacesJSON), nil
		case args[0] == "agent":
			return []byte(herdrAgentsJSON), nil
		case args[0] == "tab":
			return []byte(herdrTabsJSON), nil
		case args[0] == "pane":
			return []byte(herdrPanesJSON), nil
		}
		return nil, errors.New("unexpected args")
	}
}

func TestHerdrOverview(t *testing.T) {
	var calls [][]string
	s := herdrTestServer(t, herdrStubRunner(&calls))

	rec := do(t, s, "GET", "/api/herdr/overview", nil)
	require.Equal(t, 200, rec.Code)
	res := decode[struct {
		Available  bool `json:"available"`
		Workspaces []struct {
			WorkspaceId string `json:"workspace_id"`
			Label       string `json:"label"`
			AgentStatus string `json:"agent_status"`
			Focused     bool   `json:"focused"`
		} `json:"workspaces"`
		Agents []struct {
			Name        string `json:"name"`
			Status      string `json:"status"`
			WorkspaceId string `json:"workspace_id"`
			Cwd         string `json:"cwd"`
			Title       string `json:"title"`
		} `json:"agents"`
		Tabs []struct {
			TabId       string `json:"tab_id"`
			WorkspaceId string `json:"workspace_id"`
			Label       string `json:"label"`
			Focused     bool   `json:"focused"`
			PaneCount   int    `json:"pane_count"`
		} `json:"tabs"`
		Panes []struct {
			PaneId      string  `json:"pane_id"`
			TabId       string  `json:"tab_id"`
			Cwd         string  `json:"cwd"`
			AgentStatus string  `json:"agent_status"`
			Title       *string `json:"title"`
		} `json:"panes"`
	}](t, rec)

	assert.True(t, res.Available)
	require.Len(t, res.Workspaces, 2)
	assert.Equal(t, "working", res.Workspaces[1].AgentStatus)
	assert.True(t, res.Workspaces[1].Focused)
	require.Len(t, res.Agents, 1)
	assert.Equal(t, "opencode", res.Agents[0].Name)
	assert.Equal(t, "OC | do stuff", res.Agents[0].Title)
	assert.Equal(t, "working", res.Agents[0].Status)
	assert.Equal(t, "/tmp/test", res.Agents[0].Cwd)

	require.Len(t, res.Tabs, 2)
	assert.Equal(t, "main", res.Tabs[1].Label)
	assert.Equal(t, true, res.Tabs[1].Focused)
	assert.Equal(t, 2, res.Tabs[1].PaneCount)
	require.Len(t, res.Panes, 2)
	assert.Equal(t, "w7:p2", res.Panes[0].PaneId)
	assert.Equal(t, "/tmp/test/sub", res.Panes[0].Cwd)
	assert.Equal(t, "w7:t1", res.Panes[0].TabId)
	require.NotNil(t, res.Panes[0].Title)
	assert.Equal(t, "review logs", *res.Panes[0].Title)

	require.Len(t, calls, 4)
}

func TestHerdrOverviewUnavailable(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("herdr command not found")
	})
	rec := do(t, s, "GET", "/api/herdr/overview", nil)
	require.Equal(t, 200, rec.Code)
	res := decode[struct {
		Available bool  `json:"available"`
		Agents    []any `json:"agents"`
	}](t, rec)
	assert.False(t, res.Available)
	assert.Empty(t, res.Agents)
}

func TestHerdrReadAgent(t *testing.T) {
	var gotTarget string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		gotTarget = args[len(args)-1]
		return []byte("terminal output here"), nil
	})
	rec := do(t, s, "POST", "/api/herdr/agents/read", map[string]string{"target": "w7:p1"})
	require.Equal(t, 200, rec.Code)
	res := decode[struct {
		Output string `json:"output"`
	}](t, rec)
	assert.Equal(t, "terminal output here", res.Output)
	assert.Equal(t, "w7:p1", gotTarget)
}

func TestHerdrReadAgentRejectsBadTarget(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("should not run")
	})
	rec := do(t, s, "POST", "/api/herdr/agents/read", map[string]string{"target": "bad target; rm -rf"})
	assert.Equal(t, 500, rec.Code)
}

func TestHerdrPromptAgent(t *testing.T) {
	var gotArgs []string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte("ok"), nil
	})
	rec := do(t, s, "POST", "/api/herdr/agents/prompt", map[string]string{"target": "reviewer", "text": "please check tests"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "prompt", "reviewer", "please check tests"}, gotArgs)
}

func TestHerdrPromptAgentValidation(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("should not run")
	})
	for _, body := range []map[string]string{
		{"target": "reviewer", "text": ""},
		{"target": "", "text": "hello"},
	} {
		rec := do(t, s, "POST", "/api/herdr/agents/prompt", body)
		assert.NotEqual(t, 200, rec.Code)
	}
}

func TestHerdrSendKeysAgent(t *testing.T) {
	var gotArgs []string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte("ok"), nil
	})

	rec := do(t, s, "POST", "/api/herdr/agents/send-keys", map[string]any{
		"target": "w7:p1", "keys": []string{"Enter"},
	})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "send-keys", "w7:p1", "Enter"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/agents/send-keys", map[string]any{
		"target": "w7:p1", "keys": []string{"esc", "C-c"},
	})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "send-keys", "w7:p1", "esc", "C-c"}, gotArgs)
}

func TestHerdrSendKeysAgentValidation(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("should not run")
	})
	for _, body := range []map[string]any{
		{"target": "", "keys": []string{"Enter"}},
		{"target": "bad;id", "keys": []string{"Enter"}},
		{"target": "w7:p1", "keys": []string{}},
		{"target": "w7:p1", "keys": []string{""}},
		{"target": "w7:p1", "keys": []string{"--help"}},
	} {
		rec := do(t, s, "POST", "/api/herdr/agents/send-keys", body)
		assert.NotEqual(t, 200, rec.Code, body)
	}
}

func TestHerdrRenameAgent(t *testing.T) {
	var gotArgs []string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte("ok"), nil
	})

	rec := do(t, s, "POST", "/api/herdr/agents/rename", map[string]any{
		"target": "w7:p1", "name": "worker-1",
	})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "rename", "w7:p1", "worker-1"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/agents/rename", map[string]any{
		"target": "w7:p1", "clear": true,
	})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "rename", "w7:p1", "--clear"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/agents/rename", map[string]any{
		"target": "w7:p1", "name": "",
	})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "rename", "w7:p1", "--clear"}, gotArgs)
}

func TestHerdrRenameAgentValidation(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("should not run")
	})
	for _, body := range []map[string]any{
		{"target": "", "name": "worker"},
		{"target": "bad;id", "name": "worker"},
		{"target": "w7:p1", "name": strings.Repeat("a", 81)},
	} {
		rec := do(t, s, "POST", "/api/herdr/agents/rename", body)
		assert.NotEqual(t, 200, rec.Code, body)
	}
}

func TestHerdrTabOperations(t *testing.T) {
	var gotArgs []string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte("ok"), nil
	})

	rec := do(t, s, "POST", "/api/herdr/tabs/create", map[string]any{
		"workspace_id": "w7",
		"label":        "build",
	})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"tab", "create", "--workspace", "w7", "--label", "build"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/tabs/create", map[string]any{})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"tab", "create"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/tabs/rename", map[string]string{"tab_id": "w7:t2", "label": "review"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"tab", "rename", "w7:t2", "review"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/tabs/close", map[string]string{"tab_id": "w7:t2"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"tab", "close", "w7:t2"}, gotArgs)
}

// herdrNoActiveWorkspaceError mimics the CLI failure emitted when herdr holds
// no tabs/panes at all (the JSON error arrives on stderr).
func herdrNoActiveWorkspaceError() error {
	return &exec.ExitError{Stderr: []byte(
		`{"error":{"code":"workspace_not_found","message":"no active workspace"}}`,
	)}
}

func TestHerdrCreateTabBootstrapsWorkspaceWhenEmpty(t *testing.T) {
	var calls [][]string
	s, app := newTestServer(t, false)
	s.herdrRun = func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "tab" && args[1] == "create":
			return nil, herdrNoActiveWorkspaceError()
		case args[0] == "workspace" && args[1] == "create":
			return []byte(`{"id":"cli:workspace:create","result":{"type":"workspace_created",` +
				`"workspace":{"workspace_id":"w9","label":"test"},` +
				`"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}}`), nil
		}
		return []byte("ok"), nil
	}

	rec := do(t, s, "POST", "/api/herdr/tabs/create",
		map[string]string{"label": "build"}, "X-Project", "test")
	require.Equal(t, 200, rec.Code)

	// The failed `tab create` is retried as a workspace bootstrap whose root
	// tab inherits the requested label.
	require.Len(t, calls, 3)
	assert.Equal(t, []string{"tab", "create", "--label", "build"}, calls[0])
	assert.Equal(t, []string{"workspace", "create", "--cwd", app.Project.Path}, calls[1])
	assert.Equal(t, []string{"tab", "rename", "w9:t1", "build"}, calls[2])
}

func TestHerdrCreateTabKeepsWorkspaceFailureWithoutFallback(t *testing.T) {
	var calls [][]string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		return nil, herdrNoActiveWorkspaceError()
	})

	// An explicit workspace id must not silently bootstrap a new workspace.
	rec := do(t, s, "POST", "/api/herdr/tabs/create",
		map[string]string{"workspace_id": "w7"}, "X-Project", "test")
	assert.Equal(t, 500, rec.Code)
	require.Len(t, calls, 1)
}

func TestHerdrCreateTabWithProjectReusesMatchingWorkspace(t *testing.T) {
	var calls [][]string
	s := herdrTestServer(t, herdrStubRunner(&calls))

	rec := do(t, s, "POST", "/api/herdr/tabs/create",
		map[string]string{"project": "test", "label": "build"}, "X-Project", "test")
	require.Equal(t, 200, rec.Code)
	require.Len(t, calls, 2)
	assert.Equal(t, []string{"workspace", "list"}, calls[0])
	assert.Equal(t, []string{"tab", "create", "--workspace", "w7", "--label", "build"}, calls[1])
}

func TestHerdrCreateTabWithProjectBootstrapsWhenNoMatch(t *testing.T) {
	var calls [][]string
	s, app := newTestServer(t, false)
	s.herdrRun = func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "workspace" && args[1] == "list":
			return []byte(`{"id":"cli:workspace:list","result":{"type":"workspace_list","workspaces":[` +
				`{"active_tab_id":"w5:t1","agent_status":"unknown","focused":false,"label":"home_ex",` +
				`"number":1,"pane_count":1,"tab_count":1,"workspace_id":"w5"}]}}`), nil
		case args[0] == "workspace" && args[1] == "create":
			return []byte(`{"id":"cli:workspace:create","result":{"type":"workspace_created",` +
				`"workspace":{"workspace_id":"w9","label":"nosuch"},` +
				`"tab":{"tab_id":"w9:t1","workspace_id":"w9"}}}`), nil
		}
		return []byte("ok"), nil
	}

	rec := do(t, s, "POST", "/api/herdr/tabs/create",
		map[string]string{"project": "nosuch", "label": "build"}, "X-Project", "test")
	require.Equal(t, 200, rec.Code)

	// No workspace matches the project, so its first workspace is bootstrapped
	// and the root tab inherits the requested label.
	require.Len(t, calls, 3)
	assert.Equal(t, []string{"workspace", "list"}, calls[0])
	assert.Equal(t, []string{"workspace", "create", "--cwd", app.Project.Path}, calls[1])
	assert.Equal(t, []string{"tab", "rename", "w9:t1", "build"}, calls[2])
}

func TestHerdrTabValidation(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("should not run")
	})
	rec := do(t, s, "POST", "/api/herdr/tabs/rename", map[string]string{"tab_id": "w7:t2", "label": ""})
	assert.Equal(t, 500, rec.Code)

	rec = do(t, s, "POST", "/api/herdr/tabs/rename", map[string]string{"tab_id": "w7:t2", "label": strings.Repeat("x", 81)})
	assert.Equal(t, 500, rec.Code)
}

func TestHerdrPaneOperations(t *testing.T) {
	var gotArgs []string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = args
		return []byte("ok"), nil
	})

	rec := do(t, s, "POST", "/api/herdr/panes/split", map[string]string{"pane_id": "w7:p1", "direction": "right"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"pane", "split", "--pane", "w7:p1", "--direction", "right"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/panes/split", map[string]any{
		"pane_id": "w7:p1", "direction": "down", "cwd": "/tmp/elsewhere",
	})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"pane", "split", "--pane", "w7:p1", "--direction", "down", "--cwd", "/tmp/elsewhere"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/panes/rename", map[string]string{"pane_id": "w7:p1", "label": "logs"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"pane", "rename", "w7:p1", "logs"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/panes/read", map[string]string{"target": "w7:p2"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"pane", "read", "w7:p2"}, gotArgs)

	res := decode[struct {
		Output string `json:"output"`
	}](t, rec)
	assert.Equal(t, "ok", res.Output)

	rec = do(t, s, "POST", "/api/herdr/panes/close", map[string]string{"pane_id": "w7:p2"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"pane", "close", "w7:p2"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/panes/send-text", map[string]string{"pane_id": "w7:p2", "text": "echo hello"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"pane", "send-text", "w7:p2", "echo hello"}, gotArgs)

	rec = do(t, s, "POST", "/api/herdr/panes/send-keys", map[string]interface{}{"pane_id": "w7:p2", "keys": []string{"Enter", "C-c"}})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"pane", "send-keys", "w7:p2", "Enter", "C-c"}, gotArgs)
}

func TestHerdrPaneValidation(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("should not run")
	})
	rec := do(t, s, "POST", "/api/herdr/panes/split", map[string]string{"pane_id": "w7:p1", "direction": "left"})
	assert.Equal(t, 500, rec.Code)

	rec = do(t, s, "POST", "/api/herdr/panes/close", map[string]string{"pane_id": "bad;id"})
	assert.Equal(t, 500, rec.Code)

	rec = do(t, s, "POST", "/api/herdr/panes/send-text", map[string]string{"pane_id": "w7:p2", "text": ""})
	assert.Equal(t, 500, rec.Code)

	rec = do(t, s, "POST", "/api/herdr/panes/send-keys", map[string]interface{}{"pane_id": "w7:p2", "keys": []string{}})
	assert.Equal(t, 500, rec.Code)
}

const herdrStartFileTabCreateJSON = `{"id":"cli:tab:create","result":{"type":"tab_created",` +
	`"tab":{"tab_id":"w7:t3","workspace_id":"w7","label":"app.go"},` +
	`"root_pane":{"pane_id":"w7:p3","workspace_id":"w7"}}}`

const herdrStartFileAgentJSON = `{"id":"cli:agent:list","result":{"agents":[` +
	`{"agent":"opencode","name":"src-app-go","agent_status":"working","cwd":"/tmp/test",` +
	`"pane_id":"w7:p3","tab_id":"w7:t3","workspace_id":"w7"}]}}`

func TestHerdrStartFileAgentReusesExistingAgent(t *testing.T) {
	var calls [][]string
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		// The file's agent already exists, so nothing else may run.
		if args[0] == "agent" && args[1] == "list" {
			return []byte(herdrStartFileAgentJSON), nil
		}
		return nil, errors.New("unexpected args")
	})

	rec := do(t, s, "POST", "/api/herdr/agents/start-file", map[string]string{"path": "src/app.go"})
	require.Equal(t, 200, rec.Code)
	res := decode[struct {
		Ok    bool `json:"ok"`
		Agent struct {
			Name        string `json:"name"`
			Status      string `json:"status"`
			WorkspaceId string `json:"workspace_id"`
			PaneId      string `json:"pane_id"`
		} `json:"agent"`
	}](t, rec)
	assert.True(t, res.Ok)
	assert.Equal(t, "src-app-go", res.Agent.Name)
	assert.Equal(t, "working", res.Agent.Status)
	assert.Equal(t, "w7:p3", res.Agent.PaneId)
	require.Len(t, calls, 1)
}

func TestHerdrStartFileAgentCreatesTabAndStarts(t *testing.T) {
	var calls [][]string
	s, app := newTestServer(t, false)
	s.herdrRun = func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "agent" && args[1] == "list":
			return []byte(`{"id":"cli:agent:list","result":{"agents":[]}}`), nil
		case args[0] == "workspace" && args[1] == "list":
			return []byte(herdrWorkspacesJSON), nil
		case args[0] == "tab" && args[1] == "list":
			return []byte(herdrTabsJSON), nil
		case args[0] == "tab" && args[1] == "create":
			return []byte(herdrStartFileTabCreateJSON), nil
		case args[0] == "agent" && args[1] == "start":
			return []byte("ok"), nil
		}
		return nil, errors.New("unexpected args: " + strings.Join(args, " "))
	}

	rec := do(t, s, "POST", "/api/herdr/agents/start-file", map[string]string{"path": "src/app.go"})
	require.Equal(t, 200, rec.Code)
	res := decode[struct {
		Ok    bool `json:"ok"`
		Agent struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"agent"`
	}](t, rec)
	assert.True(t, res.Ok)
	assert.Equal(t, "src-app-go", res.Agent.Name)
	assert.Equal(t, "unknown", res.Agent.Status)

	require.Len(t, calls, 6)
	assert.Equal(t, []string{"agent", "list"}, calls[0])
	assert.Equal(t, []string{"workspace", "list"}, calls[1])
	assert.Equal(t, []string{"tab", "list", "--workspace", "w7"}, calls[2])
	assert.Equal(t, []string{"tab", "create", "--workspace", "w7", "--label", "app.go", "--cwd", app.Project.Path}, calls[3])
	assert.Equal(t, []string{"agent", "start", "src-app-go", "--kind", "opencode", "--pane", "w7:p3"}, calls[4])
}

func TestHerdrStartFileAgentHonorsKind(t *testing.T) {
	var startArgs []string
	s, app := newTestServer(t, false)
	s.herdrRun = func(ctx context.Context, args ...string) ([]byte, error) {
		switch {
		case args[0] == "agent" && args[1] == "list":
			return []byte(`{"id":"cli:agent:list","result":{"agents":[]}}`), nil
		case args[0] == "workspace" && args[1] == "list":
			return []byte(herdrWorkspacesJSON), nil
		case args[0] == "tab" && args[1] == "list":
			return []byte(herdrTabsJSON), nil
		case args[0] == "tab" && args[1] == "create":
			return []byte(herdrStartFileTabCreateJSON), nil
		case args[0] == "agent" && args[1] == "start":
			startArgs = args
			return []byte("ok"), nil
		}
		return nil, errors.New("unexpected args")
	}
	_ = app

	rec := do(t, s, "POST", "/api/herdr/agents/start-file", map[string]string{"path": "app.go", "kind": "claude"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "start", "app-go", "--kind", "claude", "--pane", "w7:p3"}, startArgs)
}

func TestHerdrStartFileAgentReusesTab(t *testing.T) {
	var calls [][]string
	var startArgs []string
	s, _ := newTestServer(t, false)
	s.herdrRun = func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "agent" && args[1] == "list":
			return []byte(`{"id":"cli:agent:list","result":{"agents":[]}}`), nil
		case args[0] == "workspace" && args[1] == "list":
			return []byte(herdrWorkspacesJSON), nil
		case args[0] == "tab" && args[1] == "list":
			return []byte(`{"id":"cli:tab:list","result":{"tabs":[` +
				`{"label":"app.go","number":1,"pane_count":1,"tab_id":"w7:t4","workspace_id":"w7"}]}}`), nil
		case args[0] == "pane" && args[1] == "list":
			return []byte(`{"id":"cli:pane:list","result":{"panes":[` +
				`{"cwd":"/tmp/test","focused":false,"pane_id":"w7:p4","tab_id":"w7:t4","workspace_id":"w7"}]}}`), nil
		case args[0] == "agent" && args[1] == "start":
			startArgs = args
			return []byte("ok"), nil
		}
		return nil, errors.New("unexpected args: " + strings.Join(args, " "))
	}

	rec := do(t, s, "POST", "/api/herdr/agents/start-file", map[string]string{"path": "app.go"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"agent", "start", "app-go", "--kind", "opencode", "--pane", "w7:p4"}, startArgs)
	for _, c := range calls {
		if len(c) >= 2 {
			assert.NotEqual(t, []string{"tab", "create"}, c)
		}
	}
}

func TestHerdrStartFileAgentSkippedOccupiedTab(t *testing.T) {
	// A tab matching the label whose pane is already running an agent is not an
	// available shell, so starting must create a fresh tab instead of targeting
	// the occupied pane.
	var calls [][]string
	var startArgs []string
	s, app := newTestServer(t, false)
	s.herdrRun = func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "agent" && args[1] == "list":
			return []byte(`{"id":"cli:agent:list","result":{"agents":[]}}`), nil
		case args[0] == "workspace" && args[1] == "list":
			return []byte(herdrWorkspacesJSON), nil
		case args[0] == "tab" && args[1] == "list":
			return []byte(`{"id":"cli:tab:list","result":{"tabs":[` +
				`{"label":"app.go","number":1,"pane_count":1,"tab_id":"w7:t4","workspace_id":"w7"}]}}`), nil
		case args[0] == "pane" && args[1] == "list":
			// The matching tab's pane already runs an opencode agent.
			return []byte(`{"id":"cli:pane:list","result":{"panes":[` +
				`{"agent":"opencode","agent_status":"idle","cwd":"/tmp/test","focused":false,` +
				`"pane_id":"w7:p4","tab_id":"w7:t4","workspace_id":"w7"}]}}`), nil
		case args[0] == "tab" && args[1] == "create":
			return []byte(herdrStartFileTabCreateJSON), nil
		case args[0] == "agent" && args[1] == "start":
			startArgs = args
			return []byte("ok"), nil
		}
		return nil, errors.New("unexpected args: " + strings.Join(args, " "))
	}

	rec := do(t, s, "POST", "/api/herdr/agents/start-file", map[string]string{"path": "app.go"})
	require.Equal(t, 200, rec.Code)
	assert.Equal(t, []string{"tab", "create", "--workspace", "w7", "--label", "app.go", "--cwd", app.Project.Path}, calls[4])
	assert.Equal(t, []string{"agent", "start", "app-go", "--kind", "opencode", "--pane", "w7:p3"}, startArgs)
}

func TestHerdrStartFileAgentBootstrapsWorkspace(t *testing.T) {
	var calls [][]string
	var startArgs []string
	s, app := newTestServer(t, false)
	s.herdrRun = func(ctx context.Context, args ...string) ([]byte, error) {
		calls = append(calls, args)
		switch {
		case args[0] == "agent" && args[1] == "list":
			return []byte(`{"id":"cli:agent:list","result":{"agents":[]}}`), nil
		case args[0] == "workspace" && args[1] == "list":
			return []byte(`{"id":"cli:workspace:list","result":{"workspaces":[]}}`), nil
		case args[0] == "workspace" && args[1] == "create":
			return []byte(`{"id":"cli:workspace:create","result":{"type":"workspace_created",` +
				`"workspace":{"workspace_id":"w9","label":"test"},` +
				`"tab":{"tab_id":"w9:t1","workspace_id":"w9"},` +
				`"root_pane":{"pane_id":"w9:p1","workspace_id":"w9"}}}`), nil
		case args[0] == "tab" && args[1] == "rename":
			return []byte("ok"), nil
		case args[0] == "agent" && args[1] == "start":
			startArgs = args
			return []byte("ok"), nil
		}
		return nil, errors.New("unexpected args: " + strings.Join(args, " "))
	}

	rec := do(t, s, "POST", "/api/herdr/agents/start-file", map[string]string{"path": "app.go"})
	require.Equal(t, 200, rec.Code)
	require.Len(t, calls, 6)
	assert.Equal(t, []string{"workspace", "list"}, calls[1])
	assert.Equal(t, []string{"workspace", "create", "--cwd", app.Project.Path}, calls[2])
	assert.Equal(t, []string{"tab", "rename", "w9:t1", "app.go"}, calls[3])
	assert.Equal(t, []string{"agent", "start", "app-go", "--kind", "opencode", "--pane", "w9:p1"}, startArgs)
}

func TestHerdrStartFileAgentValidation(t *testing.T) {
	s := herdrTestServer(t, func(ctx context.Context, args ...string) ([]byte, error) {
		return nil, errors.New("should not run")
	})
	for _, body := range []map[string]string{
		{"path": ""},
		{"path": "/etc/passwd"},
		{"path": "../escape.go"},
		{"path": "notes..md"},
		{"path": "x.go", "kind": "not-an-agent"},
	} {
		rec := do(t, s, "POST", "/api/herdr/agents/start-file", body)
		assert.Equal(t, 400, rec.Code, body)
	}

	// A filename whose slug is usable proceeds past validation; the stub
	// runner then reports no agent and the request fails with 500.
	rec := do(t, s, "POST", "/api/herdr/agents/start-file", map[string]string{"path": "has space.md"})
	assert.Equal(t, 500, rec.Code)
}

func TestHerdrFileAgentName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"app.go", "app-go"},
		{"README.md", "readme-md"},
		{"architecture.md", "architecture-md"},
		{"App.Config.json", "app-config-json"},
		{"foo_bar.txt", "foo_bar-txt"},
		{"My File (final).txt", "my-file-final-txt"},
		{"src/app.go", "src-app-go"},
		{"docs/architecture.md", "docs-architecture-md"},
		{"a/b/c.go", "b-c-go"},
		{"deep/nested/dir/file.md", "dir-file-md"},
		{"UPPERCASE", "uppercase"},
		{"123.txt", "f123-txt"},
		{"!!!", ""},
		{"a-_-b", "a-_-b"},
	} {
		tc := tc
		assert.Equal(t, tc.want, herdrFileAgentName(tc.in), tc.in)
	}

	long := strings.Repeat("x", 40) + strings.Repeat("-y", 10) + ".md"
	got := herdrFileAgentName(long)
	require.Len(t, got, 32)
	assert.Regexp(t, `^[a-z][a-z0-9_-]*$`, got)
}
