package entrypoint

import (
	"context"
	"errors"
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
		Available bool `json:"available"`
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
