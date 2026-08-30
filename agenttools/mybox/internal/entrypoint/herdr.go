package entrypoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

const herdrTimeout = 6 * time.Second

// herdrRunFunc executes the herdr CLI and returns raw stdout.
type herdrRunFunc func(ctx context.Context, args ...string) ([]byte, error)

var herdrTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)

func herdrErrorMessage(out []byte, err error) error {
	var env struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if len(out) > 0 && json.Unmarshal(out, &env) == nil && env.Error.Message != "" {
		return errors.New(env.Error.Message)
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		if json.Unmarshal(exitErr.Stderr, &env) == nil && env.Error.Message != "" {
			return errors.New(env.Error.Message)
		}
	}
	return err
}

func defaultHerdrRun(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, herdrTimeout)
	defer cancel()
	bin, err := exec.LookPath("herdr")
	if err != nil {
		return nil, fmt.Errorf("herdr command not found: %w", err)
	}
	out, err := exec.CommandContext(ctx, bin, args...).Output()
	if err != nil {
		if errMsg := herdrErrorMessage(out, err); errMsg != err {
			return nil, errMsg
		}
		return nil, fmt.Errorf("herdr %s failed: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func (s *Server) runHerdr(ctx context.Context, args ...string) ([]byte, error) {
	if s.herdrRun != nil {
		return s.herdrRun(ctx, args...)
	}
	return defaultHerdrRun(ctx, args...)
}

type herdrEnvelope struct {
	Result json.RawMessage `json:"result"`
}

type herdrWorkspaceRaw struct {
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Number      int    `json:"number"`
	AgentStatus string `json:"agent_status"`
	Focused     bool   `json:"focused"`
	TabCount    int    `json:"tab_count"`
	PaneCount   int    `json:"pane_count"`
}

type herdrWorkspaceListResult struct {
	Workspaces []herdrWorkspaceRaw `json:"workspaces"`
}

type herdrAgentRaw struct {
	Agent       string `json:"agent"`
	Name        string `json:"name"`
	AgentStatus string `json:"agent_status"`
	Cwd         string `json:"cwd"`
	Focused     bool   `json:"focused"`
	PaneID      string `json:"pane_id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"terminal_title_stripped"`
}

type herdrAgentListResult struct {
	Agents []herdrAgentRaw `json:"agents"`
}

type herdrTabRaw struct {
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Label       string `json:"label"`
	Number      int    `json:"number"`
	AgentStatus string `json:"agent_status"`
	Focused     bool   `json:"focused"`
	PaneCount   int    `json:"pane_count"`
}

type herdrTabListResult struct {
	Tabs []herdrTabRaw `json:"tabs"`
}

type herdrPaneRaw struct {
	PaneID      string `json:"pane_id"`
	TabID       string `json:"tab_id"`
	WorkspaceID string `json:"workspace_id"`
	Cwd         string `json:"cwd"`
	AgentStatus string `json:"agent_status"`
	Title       string `json:"title"`
	Focused     bool   `json:"focused"`
}

type herdrPaneListResult struct {
	Panes []herdrPaneRaw `json:"panes"`
}

// GetHerdrOverview returns herdr workspaces and agents in one call.
// When the herdr CLI or server is unavailable it reports available=false
// instead of failing so that the UI can degrade gracefully.
func (s *Server) GetHerdrOverview(w http.ResponseWriter, r *http.Request) {
	overview := &api.HerdrOverview{
		Available:  false,
		Workspaces: []api.HerdrWorkspace{},
		Agents:     []api.HerdrAgent{},
	}
	wsOut, err := s.runHerdr(r.Context(), "workspace", "list")
	if err != nil {
		writeJSONResponse(w, http.StatusOK, overview)
		return
	}
	var wsEnv herdrEnvelope
	if err := json.Unmarshal(wsOut, &wsEnv); err == nil {
		var list herdrWorkspaceListResult
		if err := json.Unmarshal(wsEnv.Result, &list); err == nil {
			for _, raw := range list.Workspaces {
				overview.Workspaces = append(overview.Workspaces, api.HerdrWorkspace{
					WorkspaceId: raw.WorkspaceID,
					Label:       raw.Label,
					Number:      &raw.Number,
					AgentStatus: raw.AgentStatus,
					Focused:     &raw.Focused,
					TabCount:    &raw.TabCount,
					PaneCount:   &raw.PaneCount,
				})
			}
			overview.Available = true
		}
	}

	agOut, err := s.runHerdr(r.Context(), "agent", "list")
	if err == nil {
		var agEnv herdrEnvelope
		if err := json.Unmarshal(agOut, &agEnv); err == nil {
			var list herdrAgentListResult
			if err := json.Unmarshal(agEnv.Result, &list); err == nil {
				for _, raw := range list.Agents {
					name := raw.Name
					if name == "" {
						name = raw.Agent
					}
					agent := api.HerdrAgent{
						Name:        name,
						Status:      raw.AgentStatus,
						WorkspaceId: raw.WorkspaceID,
						Cwd:         &raw.Cwd,
						Focused:     &raw.Focused,
						PaneId:      raw.PaneID,
					}
					if raw.Name != "" {
						customName := raw.Name
						agent.CustomName = &customName
					}
					if title := strings.TrimSpace(raw.Title); title != "" {
						agent.Title = &title
					}
					overview.Agents = append(overview.Agents, agent)
				}
			}
		}
	}

	tabOut, err := s.runHerdr(r.Context(), "tab", "list")
	if err == nil {
		var env herdrEnvelope
		if err := json.Unmarshal(tabOut, &env); err == nil {
			var list herdrTabListResult
			if err := json.Unmarshal(env.Result, &list); err == nil {
				for _, raw := range list.Tabs {
					overview.Tabs = append(overview.Tabs, api.HerdrTab{
						TabId:       raw.TabID,
						WorkspaceId: raw.WorkspaceID,
						Label:       raw.Label,
						Number:      &raw.Number,
						AgentStatus: &raw.AgentStatus,
						Focused:     &raw.Focused,
						PaneCount:   &raw.PaneCount,
					})
				}
			}
		}
	}

	paneOut, err := s.runHerdr(r.Context(), "pane", "list")
	if err == nil {
		var env herdrEnvelope
		if err := json.Unmarshal(paneOut, &env); err == nil {
			var list herdrPaneListResult
			if err := json.Unmarshal(env.Result, &list); err == nil {
				for _, raw := range list.Panes {
					pane := api.HerdrPane{
						PaneId:      raw.PaneID,
						TabId:       raw.TabID,
						WorkspaceId: raw.WorkspaceID,
						Cwd:         &raw.Cwd,
						AgentStatus: &raw.AgentStatus,
						Title:       &raw.Title,
						Focused:     &raw.Focused,
					}
					if pane.Title != nil && *pane.Title == "" {
						pane.Title = nil
					}
					overview.Panes = append(overview.Panes, pane)
				}
			}
		}
	}
	writeJSONResponse(w, http.StatusOK, overview)
}

// ReadHerdrAgent returns recent terminal output of the target agent pane.
func (s *Server) ReadHerdrAgent(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrReadRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !herdrTargetPattern.MatchString(req.Target) {
		writeError(w, fmt.Errorf("invalid agent target"))
		return
	}
	out, err := s.runHerdr(r.Context(), "agent", "read", req.Target)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrReadResponse{Output: string(out)})
}

// PromptHerdrAgent submits a prompt to the target agent.
func (s *Server) PromptHerdrAgent(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrPromptRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Target = strings.TrimSpace(req.Target)
	req.Text = strings.TrimSpace(req.Text)
	if !herdrTargetPattern.MatchString(req.Target) {
		writeError(w, fmt.Errorf("invalid agent target"))
		return
	}
	if req.Text == "" || len(req.Text) > 4096 {
		writeError(w, fmt.Errorf("prompt text must be between 1 and 4096 characters"))
		return
	}
	if _, err := s.runHerdr(r.Context(), "agent", "prompt", req.Target, req.Text); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrPromptResponse{Ok: true})
}

// validHerdrKey reports whether key looks like a key name accepted by
// `herdr send-keys` (e.g. Enter, esc, C-c, Tab). Empty and flag-like tokens
// are rejected; a lone "-" is a literal key character.
func validHerdrKey(key string) bool {
	switch {
	case key == "":
		return false
	case len(key) > 64:
		return false
	case strings.HasPrefix(key, "-") && key != "-":
		return false
	}
	return true
}

// SendKeysHerdrAgent sends key presses (e.g. Enter, esc) to the target agent.
func (s *Server) SendKeysHerdrAgent(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrAgentSendKeysRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Target = strings.TrimSpace(req.Target)
	if !herdrTargetPattern.MatchString(req.Target) {
		writeError(w, fmt.Errorf("invalid agent target"))
		return
	}
	if len(req.Keys) == 0 || len(req.Keys) > 16 {
		writeError(w, fmt.Errorf("keys must contain between 1 and 16 entries"))
		return
	}
	for _, key := range req.Keys {
		if !validHerdrKey(key) {
			writeError(w, fmt.Errorf("invalid key %q", key))
			return
		}
	}
	args := append([]string{"agent", "send-keys", req.Target}, req.Keys...)
	if _, err := s.runHerdr(r.Context(), args...); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrOpResponse{Ok: true})
}

// RenameHerdrAgent renames an agent or clears its custom name.
func (s *Server) RenameHerdrAgent(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrAgentRenameRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Target = strings.TrimSpace(req.Target)
	if !herdrTargetPattern.MatchString(req.Target) {
		writeError(w, fmt.Errorf("invalid agent target"))
		return
	}
	var args []string
	if req.Clear != nil && *req.Clear {
		args = []string{"agent", "rename", req.Target, "--clear"}
	} else if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		name, ok := validHerdrLabel(*req.Name)
		if !ok {
			writeError(w, fmt.Errorf("name must be between 1 and 80 characters"))
			return
		}
		args = []string{"agent", "rename", req.Target, name}
	} else {
		args = []string{"agent", "rename", req.Target, "--clear"}
	}
	if _, err := s.runHerdr(r.Context(), args...); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrOpResponse{Ok: true})
}



func validHerdrLabel(label string) (string, bool) {
	label = strings.TrimSpace(label)
	return label, label != "" && len(label) <= 80
}

// CreateHerdrTab creates a new tab in the given (or active) workspace.
// When herdr holds no tabs/panes at all the CLI fails with "no active
// workspace"; in that case the first workspace of the current project is
// bootstrapped instead, which also creates the requested tab.
func (s *Server) CreateHerdrTab(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrTabCreateRequest
	if !decodeBody(w, r, &req) {
		return
	}
	var label string
	if req.Label != nil && *req.Label != "" {
		l, ok := validHerdrLabel(*req.Label)
		if !ok {
			writeError(w, fmt.Errorf("label must be between 1 and 80 characters"))
			return
		}
		label = l
	}
	args := []string{"tab", "create"}
	wsRequested := false
	if req.WorkspaceId != nil && *req.WorkspaceId != "" {
		if !herdrTargetPattern.MatchString(*req.WorkspaceId) {
			writeError(w, fmt.Errorf("invalid workspace id"))
			return
		}
		wsRequested = true
		args = append(args, "--workspace", *req.WorkspaceId)
	} else if project := strings.TrimSpace(reqProject(req)); project != "" {
		// The tab belongs to a specific project; make sure a matching
		// workspace exists before creating its tab.
		wsID, handled := s.ensureHerdrProjectWorkspace(w, r, project, label, req.Cwd)
		if handled {
			return
		}
		args = append(args, "--workspace", wsID)
	}
	if label != "" {
		args = append(args, "--label", label)
	}
	if req.Cwd != nil && *req.Cwd != "" {
		cwd := strings.TrimSpace(*req.Cwd)
		args = append(args, "--cwd", cwd)
	}
	if _, err := s.runHerdr(r.Context(), args...); err != nil {
		if !wsRequested && herdrNoActiveWorkspace(err) {
			if s.bootstrapHerdrWorkspace(w, r, label, req.Cwd) {
				return
			}
		}
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrOpResponse{Ok: true})
}

// reqProject returns the optional project name of a create-tab request.
func reqProject(req api.HerdrTabCreateRequest) string {
	if req.Project == nil {
		return ""
	}
	return *req.Project
}

// ensureHerdrProjectWorkspace makes sure a herdr workspace labelled after the
// project exists and returns its id. When no workspace matches, the first
// workspace of the project is bootstrapped (which also creates a tab) and the
// response is written to the client (handled=true).
func (s *Server) ensureHerdrProjectWorkspace(
	w http.ResponseWriter,
	r *http.Request,
	project string,
	label string,
	cwd *string,
) (string, bool) {
	out, err := s.runHerdr(r.Context(), "workspace", "list")
	if err != nil {
		writeError(w, err)
		return "", true
	}
	var env herdrEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		writeError(w, fmt.Errorf("unexpected herdr workspace list output"))
		return "", true
	}
	var list herdrWorkspaceListResult
	if err := json.Unmarshal(env.Result, &list); err != nil {
		writeError(w, fmt.Errorf("unexpected herdr workspace list result"))
		return "", true
	}
	for _, raw := range list.Workspaces {
		if raw.Label != project {
			continue
		}
		if !herdrTargetPattern.MatchString(raw.WorkspaceID) {
			break
		}
		return raw.WorkspaceID, false
	}
	// No workspace carries the project label yet: bootstrap it so herdr ends
	// up with the project's first tab instead of failing on an empty server.
	s.bootstrapHerdrWorkspace(w, r, label, cwd)
	return "", true
}

// herdrNoActiveWorkspace reports whether err is herdr's "no active workspace"
// failure, which the CLI emits on stderr when no tabs/panes exist yet.
func herdrNoActiveWorkspace(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return false
	}
	msg := string(exitErr.Stderr)
	return strings.Contains(msg, "workspace_not_found") ||
		strings.Contains(msg, "no active workspace")
}

// bootstrapHerdrWorkspace creates herdr's first workspace for the current
// project so that its initial tab+pane satisfy the create-tab request.
// It reports whether a response has been written to the client.
func (s *Server) bootstrapHerdrWorkspace(w http.ResponseWriter, r *http.Request, label string, cwd *string) bool {
	wsArgs := []string{"workspace", "create"}
	cwdArg := ""
	if cwd != nil {
		cwdArg = strings.TrimSpace(*cwd)
	}
	if cwdArg == "" {
		if app, appErr := s.getApp(r); appErr == nil && app.Project != nil {
			cwdArg = app.Project.Path
		}
	}
	if cwdArg != "" {
		wsArgs = append(wsArgs, "--cwd", cwdArg)
	}
	out, err := s.runHerdr(r.Context(), wsArgs...)
	if err != nil {
		writeError(w, err)
		return true
	}
	// The bootstrap workspace ships with a numbered root tab; apply the
	// requested tab label to it when one was given.
	if label != "" {
		if id := herdrCreatedTabID(out); id != "" {
			if _, err := s.runHerdr(r.Context(), "tab", "rename", id, label); err != nil {
				writeError(w, err)
				return true
			}
		}
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrOpResponse{Ok: true})
	return true
}

// herdrCreatedTabID extracts the root tab id from `herdr workspace create`
// output, or "" when it cannot be found.
func herdrCreatedTabID(out []byte) string {
	var env struct {
		Result struct {
			Tab struct {
				TabID string `json:"tab_id"`
			} `json:"tab"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		return ""
	}
	if !herdrTargetPattern.MatchString(env.Result.Tab.TabID) {
		return ""
	}
	return env.Result.Tab.TabID
}

func herdrIDOp(w http.ResponseWriter, r *http.Request, kind string, id string, run func(ctx context.Context, args ...string) ([]byte, error), build func(id string) []string) {
	if !herdrTargetPattern.MatchString(id) {
		writeError(w, fmt.Errorf("invalid %s id", kind))
		return
	}
	if _, err := run(r.Context(), build(id)...); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrOpResponse{Ok: true})
}

// RenameHerdrTab renames a tab.
func (s *Server) RenameHerdrTab(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrTabRenameRequest
	if !decodeBody(w, r, &req) {
		return
	}
	label, ok := validHerdrLabel(req.Label)
	if !ok {
		writeError(w, fmt.Errorf("label must be between 1 and 80 characters"))
		return
	}
	herdrIDOp(w, r, "tab", req.TabId, s.runHerdr, func(id string) []string {
		return []string{"tab", "rename", id, label}
	})
}

// CloseHerdrTab closes a tab (closing the last tab closes its workspace).
func (s *Server) CloseHerdrTab(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrTabCloseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	herdrIDOp(w, r, "tab", req.TabId, s.runHerdr, func(id string) []string {
		return []string{"tab", "close", id}
	})
}

// SplitHerdrPane splits a pane to the right or below.
func (s *Server) SplitHerdrPane(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrPaneSplitRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Direction != api.Right && req.Direction != api.Down {
		writeError(w, fmt.Errorf("direction must be right or down"))
		return
	}
	herdrIDOp(w, r, "pane", req.PaneId, s.runHerdr, func(id string) []string {
		args := []string{"pane", "split", "--pane", id, "--direction", string(req.Direction)}
		if req.Cwd != nil && strings.TrimSpace(*req.Cwd) != "" {
			args = append(args, "--cwd", strings.TrimSpace(*req.Cwd))
		}
		return args
	})
}

// RenameHerdrPane renames a pane.
func (s *Server) RenameHerdrPane(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrPaneRenameRequest
	if !decodeBody(w, r, &req) {
		return
	}
	label, ok := validHerdrLabel(req.Label)
	if !ok {
		writeError(w, fmt.Errorf("label must be between 1 and 80 characters"))
		return
	}
	herdrIDOp(w, r, "pane", req.PaneId, s.runHerdr, func(id string) []string {
		return []string{"pane", "rename", id, label}
	})
}

// ReadHerdrPane returns recent terminal output of a pane.
func (s *Server) ReadHerdrPane(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrReadRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if !herdrTargetPattern.MatchString(req.Target) {
		writeError(w, fmt.Errorf("invalid pane id"))
		return
	}
	out, err := s.runHerdr(r.Context(), "pane", "read", req.Target)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrReadResponse{Output: string(out)})
}

// CloseHerdrPane closes a pane.
func (s *Server) CloseHerdrPane(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrPaneCloseRequest
	if !decodeBody(w, r, &req) {
		return
	}
	herdrIDOp(w, r, "pane", req.PaneId, s.runHerdr, func(id string) []string {
		return []string{"pane", "close", id}
	})
}

// SendTextHerdrPane sends literal text to a pane.
func (s *Server) SendTextHerdrPane(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrPaneSendTextRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Text == "" {
		writeError(w, fmt.Errorf("text cannot be empty"))
		return
	}
	herdrIDOp(w, r, "pane", req.PaneId, s.runHerdr, func(id string) []string {
		return []string{"pane", "send-text", id, req.Text}
	})
}

// SendKeysHerdrPane sends key presses to a pane.
func (s *Server) SendKeysHerdrPane(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrPaneSendKeysRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Keys) == 0 {
		writeError(w, fmt.Errorf("keys cannot be empty"))
		return
	}
	herdrIDOp(w, r, "pane", req.PaneId, s.runHerdr, func(id string) []string {
		args := []string{"pane", "send-keys", id}
		args = append(args, req.Keys...)
		return args
	})
}
