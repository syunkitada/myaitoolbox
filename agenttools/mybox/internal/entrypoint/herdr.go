package entrypoint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/labstack/echo/v4"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
)

const (
	herdrTimeout      = 6 * time.Second
	herdrMaxReadLines = 5000
)

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
	Agent       string `json:"agent"`
	AgentStatus string `json:"agent_status"`
	Title       string `json:"title"`
	Focused     bool   `json:"focused"`
}

const (
	herdrPaneReadyTimeout   = 5 * time.Second
	herdrPanePollInterval   = 50 * time.Millisecond
	herdrAgentStartAttempts = 3
)

type herdrPaneForegroundProcessRaw struct {
	PID  int      `json:"pid"`
	Name string   `json:"name"`
	Argv []string `json:"argv"`
}

type herdrPaneProcessInfoRaw struct {
	ShellPID            int                             `json:"shell_pid"`
	ForegroundProcesses []herdrPaneForegroundProcessRaw `json:"foreground_processes"`
}

func herdrPaneProcessInfoReady(out []byte) (bool, error) {
	var env struct {
		Result struct {
			ProcessInfo herdrPaneProcessInfoRaw `json:"process_info"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &env); err != nil {
		return false, fmt.Errorf("unexpected herdr pane process info output")
	}
	if env.Result.ProcessInfo.ShellPID <= 0 {
		return false, nil
	}
	processes := env.Result.ProcessInfo.ForegroundProcesses
	return len(processes) == 1 &&
		processes[0].PID == env.Result.ProcessInfo.ShellPID &&
		herdrForegroundProcessIsShell(processes[0]), nil
}

func herdrForegroundProcessIsShell(process herdrPaneForegroundProcessRaw) bool {
	name := strings.TrimPrefix(filepath.Base(strings.TrimSpace(process.Name)), "-")
	if name == "" && len(process.Argv) > 0 {
		name = strings.TrimPrefix(filepath.Base(strings.TrimSpace(process.Argv[0])), "-")
	}
	if name == "" {
		return false
	}
	if shell := filepath.Base(strings.TrimSpace(os.Getenv("SHELL"))); shell != "" && name == shell {
		return true
	}
	switch name {
	case "ash", "bash", "csh", "dash", "fish", "ksh", "mksh", "nu", "sh", "tcsh", "yash", "zsh":
		return true
	default:
		return false
	}
}

type herdrPaneListResult struct {
	Panes []herdrPaneRaw `json:"panes"`
}

func attachHerdrTask(agent *api.HerdrAgent, task *domain.Task) {
	if agent == nil || task == nil {
		return
	}
	taskID := task.ID
	taskTitle := task.Title
	agent.TaskId = &taskID
	agent.TaskTitle = &taskTitle
}

func (s *Server) enrichHerdrAgentsWithTasks(ctx context.Context, app *App, agents []api.HerdrAgent) {
	if app == nil || app.Tasks == nil {
		return
	}
	tasks, err := app.Tasks.List(ctx, application.TaskFilter{})
	if err != nil {
		return
	}
	byAgentName := make(map[string]*domain.Task, len(tasks))
	for i := range tasks {
		task := &tasks[i]
		if name := herdrTaskAgentName(task.ID); name != "" {
			byAgentName[name] = task
		}
	}
	for i := range agents {
		attachHerdrTask(&agents[i], byAgentName[agents[i].Name])
	}
}

// GetHerdrOverview returns herdr workspaces and agents in one call.
// When the herdr CLI or server is unavailable it reports available=false
// instead of failing so that the UI can degrade gracefully.
func (s *Server) GetHerdrOverview(w http.ResponseWriter, r *http.Request) {
	overview := &api.HerdrOverview{
		Available:  false,
		Workspaces: []api.HerdrWorkspace{},
		Agents:     []api.HerdrAgent{},
		Tabs:       []api.HerdrTab{},
		Panes:      []api.HerdrPane{},
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
	if app, err := s.getApp(r); err == nil {
		s.enrichHerdrAgentsWithTasks(r.Context(), app, overview.Agents)
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
	args, err := herdrReadArgs("agent", req.Target, req.Lines)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := s.runHerdr(r.Context(), args...)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrReadResponse{Output: string(out)})
}

// FocusHerdrAgent marks the target agent's result as seen in herdr.
func (s *Server) FocusHerdrAgent(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrReadRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Target = strings.TrimSpace(req.Target)
	if !herdrTargetPattern.MatchString(req.Target) {
		writeError(w, fmt.Errorf("invalid agent target"))
		return
	}
	if _, err := s.runHerdr(r.Context(), "agent", "focus", req.Target); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrOpResponse{Ok: true})
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
	return label, label != "" && utf8.RuneCountInString(label) <= 80
}

func validHerdrTaskTitle(title string) (string, bool) {
	title, ok := validHerdrLabel(title)
	if !ok {
		return title, false
	}
	for _, r := range title {
		if unicode.IsControl(r) {
			return title, false
		}
	}
	return title, true
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
	args, err := herdrReadArgs("pane", req.Target, req.Lines)
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := s.runHerdr(r.Context(), args...)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrReadResponse{Output: string(out)})
}

func herdrReadArgs(kind, target string, lines *int) ([]string, error) {
	args := []string{kind, "read", target}
	if lines == nil {
		return args, nil
	}
	if *lines < 1 || *lines > herdrMaxReadLines {
		return nil, fmt.Errorf("%w: line count must be between 1 and %d", domain.ErrInvalidArgument, herdrMaxReadLines)
	}
	return append(args, "--lines", strconv.Itoa(*lines)), nil
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

// HerdrPaneRect is the terminal-cell rectangle of a pane or layout area.
type HerdrPaneRect struct {
	Height int `json:"height"`
	Width  int `json:"width"`
	X      int `json:"x"`
	Y      int `json:"y"`
}

// HerdrLayoutPane is one pane position inside a tab layout, in terminal cells.
type HerdrLayoutPane struct {
	Focused bool          `json:"focused"`
	PaneID  string        `json:"pane_id"`
	Rect    HerdrPaneRect `json:"rect"`
}

// HerdrSplit describes one split edge of a tab layout (tmux-style binary tree).
type HerdrSplit struct {
	Direction string        `json:"direction"`
	ID        string        `json:"id"`
	Ratio     float64       `json:"ratio"`
	Rect      HerdrPaneRect `json:"rect"`
}

// HerdrLayout reproduces the pane structure of one herdr tab so the web UI can
// draw panes at their real relative positions and sizes.
type HerdrLayout struct {
	Area          HerdrPaneRect     `json:"area"`
	FocusedPaneID string            `json:"focused_pane_id"`
	Panes         []HerdrLayoutPane `json:"panes"`
	Splits        []HerdrSplit      `json:"splits"`
	TabID         string            `json:"tab_id"`
	WorkspaceID   string            `json:"workspace_id"`
	Zoomed        bool              `json:"zoomed"`
}

type herdrRectRaw struct {
	Height int `json:"height"`
	Width  int `json:"width"`
	X      int `json:"x"`
	Y      int `json:"y"`
}

type herdrLayoutPaneRaw struct {
	Focused bool         `json:"focused"`
	PaneID  string       `json:"pane_id"`
	Rect    herdrRectRaw `json:"rect"`
}

type herdrSplitRaw struct {
	Direction string       `json:"direction"`
	ID        string       `json:"id"`
	Ratio     float64      `json:"ratio"`
	Rect      herdrRectRaw `json:"rect"`
}

type herdrLayoutRaw struct {
	Area          herdrRectRaw         `json:"area"`
	FocusedPaneID string               `json:"focused_pane_id"`
	Panes         []herdrLayoutPaneRaw `json:"panes"`
	Splits        []herdrSplitRaw      `json:"splits"`
	TabID         string               `json:"tab_id"`
	WorkspaceID   string               `json:"workspace_id"`
	Zoomed        bool                 `json:"zoomed"`
}

type herdrLayoutListResponse struct {
	Layouts []HerdrLayout `json:"layouts"`
}

func toHerdrPaneRect(raw herdrRectRaw) HerdrPaneRect {
	return HerdrPaneRect(raw)
}

// ListHerdrLayouts returns the per-tab pane layout parsed from the `herdr api
// snapshot`, letting the web UI reproduce herdr's split structure (which panes
// sit side by side, which are stacked, and their relative sizes). When the CLI
// or its server is unavailable it reports an empty list so the UI falls back
// to its default grid instead of failing.
func (s *Server) ListHerdrLayouts(w http.ResponseWriter, r *http.Request) {
	resp := herdrLayoutListResponse{Layouts: []HerdrLayout{}}
	out, err := s.runHerdr(r.Context(), "api", "snapshot")
	if err != nil {
		writeJSONResponse(w, http.StatusOK, resp)
		return
	}
	var env herdrEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		writeJSONResponse(w, http.StatusOK, resp)
		return
	}
	var snap struct {
		Snapshot struct {
			Layouts []herdrLayoutRaw `json:"layouts"`
		} `json:"snapshot"`
	}
	if err := json.Unmarshal(env.Result, &snap); err != nil {
		writeJSONResponse(w, http.StatusOK, resp)
		return
	}
	for _, raw := range snap.Snapshot.Layouts {
		layout := HerdrLayout{
			Area:          toHerdrPaneRect(raw.Area),
			FocusedPaneID: raw.FocusedPaneID,
			Panes:         []HerdrLayoutPane{},
			Splits:        []HerdrSplit{},
			TabID:         raw.TabID,
			WorkspaceID:   raw.WorkspaceID,
			Zoomed:        raw.Zoomed,
		}
		for _, p := range raw.Panes {
			layout.Panes = append(layout.Panes, HerdrLayoutPane{
				Focused: p.Focused,
				PaneID:  p.PaneID,
				Rect:    toHerdrPaneRect(p.Rect),
			})
		}
		for _, sp := range raw.Splits {
			layout.Splits = append(layout.Splits, HerdrSplit{
				Direction: sp.Direction,
				ID:        sp.ID,
				Ratio:     sp.Ratio,
				Rect:      toHerdrPaneRect(sp.Rect),
			})
		}
		resp.Layouts = append(resp.Layouts, layout)
	}
	writeJSONResponse(w, http.StatusOK, resp)
}

// herdrPaneResizeRequest resizes a pane's edge by a number of cells.
type herdrPaneResizeRequest struct {
	// PaneID is the pane whose edge is moved.
	PaneID string `json:"pane_id"`
	// Direction is the direction of the edge being moved: left, right, up or
	// down. "right" on pane A grows pane A to the right (stealing cells from
	// its right neighbor).
	Direction string `json:"direction"`
	// Amount is the number of cells (or fraction) to move the edge.
	Amount float64 `json:"amount"`
}

var herdrResizeDirections = map[string]bool{
	"left": true, "right": true, "up": true, "down": true,
}

// ResizeHerdrPane moves a pane's edge so the web UI can keep pane sizes in
// sync with herdr by driving `herdr pane resize`.
func (s *Server) ResizeHerdrPane(w http.ResponseWriter, r *http.Request) {
	var req herdrPaneResizeRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Direction = strings.ToLower(strings.TrimSpace(req.Direction))
	if !herdrTargetPattern.MatchString(req.PaneID) {
		writeError(w, fmt.Errorf("invalid pane id"))
		return
	}
	if !herdrResizeDirections[req.Direction] {
		writeError(w, fmt.Errorf("direction must be left, right, up or down"))
		return
	}
	if math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) || req.Amount <= 0 || req.Amount > 1000 {
		writeError(w, fmt.Errorf("amount must be a positive number"))
		return
	}
	if _, err := s.runHerdr(
		r.Context(),
		"pane", "resize",
		"--pane", req.PaneID,
		"--direction", req.Direction,
		"--amount", strconv.FormatFloat(req.Amount, 'f', -1, 64),
	); err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrOpResponse{Ok: true})
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

// herdrAgentKinds lists the agent executables herdr can start in a pane.
// The set mirrors `herdr agent start --help` (possible values for --kind).
var herdrAgentKinds = map[string]bool{
	"pi": true, "claude": true, "codex": true, "gemini": true, "cursor": true,
	"devin": true, "agy": true, "cline": true, "omp": true, "mastracode": true,
	"opencode": true, "copilot": true, "kimi": true, "kiro": true, "droid": true,
	"amp": true, "grok": true, "hermes": true, "kilo": true, "qodercli": true,
	"qwen": true, "maki": true,
}

func validHerdrAgentKind(kind string) bool {
	return herdrAgentKinds[kind]
}

// herdrTaskDirFromPath validates that a project-relative file path lives
// inside a task directory (_tasks/<dir>/...) and returns the task directory
// name. Files outside a task directory cannot start a herdr agent.
func herdrTaskDirFromPath(path string) (string, bool) {
	parts := strings.Split(strings.TrimSpace(path), "/")
	if len(parts) < 2 || parts[0] != "_tasks" {
		return "", false
	}
	dir := parts[1]
	if !validHerdrTaskDir(dir) {
		return "", false
	}
	for _, p := range parts[2:] {
		if p == "" || p == "." || p == ".." {
			return "", false
		}
	}
	return dir, true
}

// validHerdrTaskDir accepts task directory names that can be represented as a
// single safe path component and a herdr tab label. Task IDs generated from
// Japanese names contain non-ASCII letters, while whitespace and path
// separators must still be rejected.
func validHerdrTaskDir(dir string) bool {
	if dir == "" || dir == "." || dir == ".." || utf8.RuneCountInString(dir) > 80 {
		return false
	}
	for _, r := range dir {
		if unicode.IsSpace(r) || unicode.IsControl(r) || r == '/' || r == '\\' {
			return false
		}
	}
	return true
}

// herdrTaskAgentName derives a herdr agent name from a task directory name
// (e.g. "20260919_foo" -> "f20260919_foo"). herdr agent names must match
// [a-z][a-z0-9_-]{0,31} (lowercase letters/digits, hyphens, underscores) while
// task directory names can carry uppercase and other characters. ASCII names
// retain their existing folded form; names containing non-ASCII characters get
// a deterministic 64-bit hash suffix to minimize collisions.
// It returns "" when no usable name can be produced.
func herdrTaskAgentName(dir string) string {
	normalized := strings.TrimSpace(strings.ToLower(dir))
	slug := normalized
	nonASCII := false
	var b strings.Builder
	prevDash := false
	for _, r := range slug {
		if r > unicode.MaxASCII {
			nonASCII = true
		}
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '_':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	slug = strings.Trim(b.String(), "-")
	if slug == "" {
		if !nonASCII {
			return ""
		}
		slug = "task"
	}
	if slug[0] < 'a' || slug[0] > 'z' {
		slug = "f" + slug
	}
	if nonASCII {
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(normalized))
		suffix := fmt.Sprintf("%016x", hash.Sum64())
		maxBaseLen := 32 - len(suffix) - 1
		if len(slug) > maxBaseLen {
			slug = strings.TrimRight(slug[:maxBaseLen], "-")
		}
		slug += "-" + suffix
	} else if len(slug) > 32 {
		slug = strings.TrimRight(slug[:32], "-")
	}
	return slug
}

type herdrStartFileAgentRequest struct {
	// Path is the project-relative file the agent should work on. It must live
	// inside a task directory (_tasks/<dir>/...): the agent is bound to that
	// task directory, not to the individual file.
	Path string `json:"path"`
	// Kind selects the agent executable herdr launches (default: opencode).
	Kind string `json:"kind"`
}

// herdrFileAgentResponse reports the agent dedicated to a task directory.
type herdrFileAgentResponse struct {
	Ok    bool            `json:"ok"`
	Agent *api.HerdrAgent `json:"agent,omitempty"`
}

// herdrCreatedTabParse extracts the workspace/tab/root-pane ids from `tab
// create` and `workspace create` outputs (both return .result.tab and
// .result.root_pane; only workspace create returns .result.workspace).
func herdrCreatedTabParse(out []byte) (wsID, tabID, paneID string) {
	var env struct {
		Result struct {
			Workspace struct {
				WorkspaceID string `json:"workspace_id"`
			} `json:"workspace"`
			Tab struct {
				TabID       string `json:"tab_id"`
				WorkspaceID string `json:"workspace_id"`
			} `json:"tab"`
			RootPane struct {
				PaneID string `json:"pane_id"`
			} `json:"root_pane"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &env) != nil {
		return "", "", ""
	}
	wsID = env.Result.Workspace.WorkspaceID
	if wsID == "" {
		wsID = env.Result.Tab.WorkspaceID
	}
	tabID = env.Result.Tab.TabID
	paneID = env.Result.RootPane.PaneID
	if !herdrTargetPattern.MatchString(wsID) {
		wsID = ""
	}
	if !herdrTargetPattern.MatchString(tabID) {
		tabID = ""
	}
	if !herdrTargetPattern.MatchString(paneID) {
		paneID = ""
	}
	return wsID, tabID, paneID
}

// herdrProjectWorkspaceID returns the id of the herdr workspace labelled after
// the project, or "" when no workspace matches. An empty id means the
// workspace has to be bootstrapped before a tab can be created.
func (s *Server) herdrProjectWorkspaceID(ctx context.Context, project string) (string, error) {
	out, err := s.runHerdr(ctx, "workspace", "list")
	if err != nil {
		return "", err
	}
	var env herdrEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return "", fmt.Errorf("unexpected herdr workspace list output")
	}
	var list herdrWorkspaceListResult
	if err := json.Unmarshal(env.Result, &list); err != nil {
		return "", fmt.Errorf("unexpected herdr workspace list result")
	}
	for _, raw := range list.Workspaces {
		if raw.Label == project && herdrTargetPattern.MatchString(raw.WorkspaceID) {
			return raw.WorkspaceID, nil
		}
	}
	return "", nil
}

// herdrFileTab looks up an existing tab whose label matches the task title and
// whose pane is still at an interactive shell prompt, so that
// restarting an agent reuses the previous tab instead of stacking duplicates.
// It returns the tab id and its pane id; both are "" when none matches (a
// matching tab whose pane already runs an agent is deliberately skipped so the
// caller creates a fresh tab rather than targeting a non-shell pane).
func (s *Server) herdrFileTab(ctx context.Context, wsID, label string) (string, string, error) {
	out, err := s.runHerdr(ctx, "tab", "list", "--workspace", wsID)
	if err != nil {
		return "", "", err
	}
	var env herdrEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return "", "", fmt.Errorf("unexpected herdr tab list output")
	}
	var list herdrTabListResult
	if err := json.Unmarshal(env.Result, &list); err != nil {
		return "", "", fmt.Errorf("unexpected herdr tab list result")
	}
	for _, raw := range list.Tabs {
		if raw.Label != label || !herdrTargetPattern.MatchString(raw.TabID) {
			continue
		}
		paneID, ok, err := s.firstHerdrTabPane(ctx, raw.TabID)
		if err != nil {
			return "", "", err
		}
		if ok {
			return raw.TabID, paneID, nil
		}
	}
	return "", "", nil
}

// firstHerdrTabPane returns the pane id of the first pane attached to a tab
// that is still at an interactive shell prompt (i.e. not already running an
// agent). ok is false when no such pane exists.
func (s *Server) firstHerdrTabPane(ctx context.Context, tabID string) (string, bool, error) {
	panes, err := s.herdrPanesForTab(ctx, tabID)
	if err != nil {
		return "", false, err
	}
	for _, raw := range panes {
		if raw.Agent != "" {
			continue
		}
		ready, err := s.herdrPaneProcessInfoReady(ctx, raw.PaneID)
		if err != nil {
			return "", false, err
		}
		if ready {
			return raw.PaneID, true, nil
		}
	}
	return "", false, nil
}

func (s *Server) herdrPanesForTab(ctx context.Context, tabID string) ([]herdrPaneRaw, error) {
	paneOut, err := s.runHerdr(ctx, "pane", "list")
	if err != nil {
		return nil, err
	}
	var env herdrEnvelope
	if err := json.Unmarshal(paneOut, &env); err != nil {
		return nil, fmt.Errorf("unexpected herdr pane list output")
	}
	var list herdrPaneListResult
	if err := json.Unmarshal(env.Result, &list); err != nil {
		return nil, fmt.Errorf("unexpected herdr pane list result")
	}
	panes := make([]herdrPaneRaw, 0, len(list.Panes))
	for _, raw := range list.Panes {
		if raw.TabID == tabID && herdrTargetPattern.MatchString(raw.PaneID) {
			panes = append(panes, raw)
		}
	}
	return panes, nil
}

func (s *Server) herdrPaneProcessInfoReady(ctx context.Context, paneID string) (bool, error) {
	out, err := s.runHerdr(ctx, "pane", "process-info", "--pane", paneID)
	if err != nil {
		return false, err
	}
	return herdrPaneProcessInfoReady(out)
}

// waitForHerdrShellPane waits until the pane's interactive shell owns the
// foreground. Herdr can return a newly-created pane before the shell has
// finished startup, and `agent start` rejects that transient state as busy.
func (s *Server) waitForHerdrShellPane(ctx context.Context, paneID string) error {
	waitCtx, cancel := context.WithTimeout(ctx, herdrPaneReadyTimeout)
	defer cancel()

	ticker := time.NewTicker(herdrPanePollInterval)
	defer ticker.Stop()
	for {
		ready, err := s.herdrPaneProcessInfoReady(waitCtx, paneID)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("herdr pane %q did not become an available shell: %w", paneID, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func herdrPaneUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "target pane") &&
		(strings.Contains(message, "not available") || strings.Contains(message, "not an available shell"))
}

func (s *Server) startHerdrAgentOnPane(ctx context.Context, name, kind, paneID string) error {
	startCtx, cancel := context.WithTimeout(ctx, herdrPaneReadyTimeout+herdrTimeout)
	defer cancel()

	var lastErr error
	for attempt := 0; attempt < herdrAgentStartAttempts; attempt++ {
		if err := s.waitForHerdrShellPane(startCtx, paneID); err != nil {
			return err
		}
		if _, err := s.runHerdr(startCtx, "agent", "start", name, "--kind", kind, "--pane", paneID); err == nil {
			return nil
		} else {
			lastErr = err
			if !herdrPaneUnavailableError(err) || attempt == herdrAgentStartAttempts-1 {
				return err
			}
		}

		select {
		case <-startCtx.Done():
			return startCtx.Err()
		case <-time.After(herdrPanePollInterval):
		}
	}
	return lastErr
}

// herdrAgentPaneForTab returns an interactive shell pane in the requested
// tab, creating a tab or splitting an occupied tab when necessary.
func (s *Server) herdrAgentPaneForTab(ctx context.Context, wsID, label, cwd string) (string, error) {
	tabOut, err := s.runHerdr(ctx, "tab", "list", "--workspace", wsID)
	if err != nil {
		return "", err
	}
	var env herdrEnvelope
	if err := json.Unmarshal(tabOut, &env); err != nil {
		return "", fmt.Errorf("unexpected herdr tab list output")
	}
	var list herdrTabListResult
	if err := json.Unmarshal(env.Result, &list); err != nil {
		return "", fmt.Errorf("unexpected herdr tab list result")
	}
	sort.SliceStable(list.Tabs, func(i, j int) bool {
		return list.Tabs[i].Number < list.Tabs[j].Number
	})

	var occupiedPaneID string
	for _, tab := range list.Tabs {
		if tab.Label != label || !herdrTargetPattern.MatchString(tab.TabID) {
			continue
		}
		panes, err := s.herdrPanesForTab(ctx, tab.TabID)
		if err != nil {
			return "", err
		}
		for _, pane := range panes {
			if pane.Agent == "" {
				ready, err := s.herdrPaneProcessInfoReady(ctx, pane.PaneID)
				if err != nil {
					return "", err
				}
				if ready {
					return pane.PaneID, nil
				}
			}
			if occupiedPaneID == "" {
				occupiedPaneID = pane.PaneID
			}
		}
	}
	if occupiedPaneID != "" {
		out, err := s.runHerdr(ctx, "pane", "split", "--pane", occupiedPaneID, "--direction", "right", "--cwd", cwd, "--no-focus")
		if err != nil {
			return "", err
		}
		paneID := herdrSplitPaneID(out)
		if paneID == "" {
			return "", fmt.Errorf("herdr pane split returned no pane")
		}
		return paneID, nil
	}

	out, err := s.runHerdr(ctx, "tab", "create", "--workspace", wsID, "--label", label, "--cwd", cwd, "--no-focus")
	if err != nil {
		return "", err
	}
	_, _, paneID := herdrCreatedTabParse(out)
	if paneID == "" {
		return "", fmt.Errorf("herdr tab create returned no root pane")
	}
	return paneID, nil
}

func herdrSplitPaneID(out []byte) string {
	var env struct {
		Result struct {
			Pane struct {
				PaneID string `json:"pane_id"`
			} `json:"pane"`
		} `json:"result"`
	}
	if json.Unmarshal(out, &env) != nil || !herdrTargetPattern.MatchString(env.Result.Pane.PaneID) {
		return ""
	}
	return env.Result.Pane.PaneID
}

// herdrCreateFileTab creates a tab labelled after the task title in the
// given workspace and returns the id of its fresh root pane. The root pane
// starts at an interactive shell prompt, which `herdr agent start` requires.
func (s *Server) herdrCreateFileTab(ctx context.Context, wsID, label, cwd string) (string, error) {
	args := []string{"tab", "create", "--workspace", wsID, "--label", label}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	out, err := s.runHerdr(ctx, args...)
	if err != nil {
		return "", err
	}
	_, _, paneID := herdrCreatedTabParse(out)
	if paneID == "" {
		return "", fmt.Errorf("herdr tab create returned no root pane")
	}
	return paneID, nil
}

// findHerdrAgent returns the live agent whose name matches, or nil.
func (s *Server) findHerdrAgent(ctx context.Context, name string) *api.HerdrAgent {
	out, err := s.runHerdr(ctx, "agent", "list")
	if err != nil {
		return nil
	}
	var env herdrEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil
	}
	var list herdrAgentListResult
	if err := json.Unmarshal(env.Result, &list); err != nil {
		return nil
	}
	for _, raw := range list.Agents {
		agentName := raw.Name
		if agentName == "" {
			agentName = raw.Agent
		}
		if agentName != name {
			continue
		}
		agent := api.HerdrAgent{
			Name:        name,
			Status:      raw.AgentStatus,
			WorkspaceId: raw.WorkspaceID,
			PaneId:      raw.PaneID,
		}
		if raw.Cwd != "" {
			cwd := raw.Cwd
			agent.Cwd = &cwd
		}
		return &agent
	}
	return nil
}

type herdrTaskLink struct {
	agentName string
	paneID    string
	tabID     string
	tabLabel  string
}

// findHerdrTaskLink resolves the live Herdr objects associated with a task's
// derived agent name. An unavailable Herdr instance is treated as no link so
// task operations remain usable without Herdr; once an agent is found, an
// incomplete topology is an error because renaming only part of the link
// would leave the task inconsistent.
func (s *Server) findHerdrTaskLink(ctx context.Context, taskID string) (*herdrTaskLink, error) {
	agentName := herdrTaskAgentName(taskID)
	if agentName == "" {
		return nil, nil
	}
	agent := s.findHerdrAgent(ctx, agentName)
	if agent == nil {
		return nil, nil
	}
	if !herdrTargetPattern.MatchString(agent.PaneId) {
		return nil, fmt.Errorf("linked Herdr agent %q has no valid pane", agentName)
	}

	paneOut, err := s.runHerdr(ctx, "pane", "list")
	if err != nil {
		return nil, fmt.Errorf("list Herdr panes for task %s: %w", taskID, err)
	}
	var paneEnv herdrEnvelope
	if err := json.Unmarshal(paneOut, &paneEnv); err != nil {
		return nil, fmt.Errorf("decode Herdr panes for task %s: %w", taskID, err)
	}
	var panes herdrPaneListResult
	if err := json.Unmarshal(paneEnv.Result, &panes); err != nil {
		return nil, fmt.Errorf("decode Herdr pane list for task %s: %w", taskID, err)
	}
	tabID := ""
	for _, pane := range panes.Panes {
		if pane.PaneID == agent.PaneId {
			tabID = pane.TabID
			break
		}
	}
	if !herdrTargetPattern.MatchString(tabID) {
		return nil, fmt.Errorf("linked Herdr agent %q has no valid tab", agentName)
	}

	tabOut, err := s.runHerdr(ctx, "tab", "list")
	if err != nil {
		return nil, fmt.Errorf("list Herdr tabs for task %s: %w", taskID, err)
	}
	var tabEnv herdrEnvelope
	if err := json.Unmarshal(tabOut, &tabEnv); err != nil {
		return nil, fmt.Errorf("decode Herdr tabs for task %s: %w", taskID, err)
	}
	var tabs herdrTabListResult
	if err := json.Unmarshal(tabEnv.Result, &tabs); err != nil {
		return nil, fmt.Errorf("decode Herdr tab list for task %s: %w", taskID, err)
	}
	for _, tab := range tabs.Tabs {
		if tab.TabID == tabID {
			return &herdrTaskLink{
				agentName: agentName,
				paneID:    agent.PaneId,
				tabID:     tabID,
				tabLabel:  tab.Label,
			}, nil
		}
	}
	return nil, fmt.Errorf("linked Herdr agent %q has no matching tab", agentName)
}

func (s *Server) renameLinkedHerdrTask(ctx context.Context, link *herdrTaskLink, newTaskID string) error {
	if link == nil {
		return nil
	}
	newAgentName := herdrTaskAgentName(newTaskID)
	if newAgentName == "" {
		return fmt.Errorf("task %q does not produce a valid Herdr agent name", newTaskID)
	}

	agentRenamed := false
	tabRenamed := false
	tryErr := func(err error) error {
		rollbackErrors := []string{}
		if tabRenamed {
			label := strings.TrimSpace(link.tabLabel)
			if label == "" {
				label = link.agentName
			}
			if _, rollbackErr := s.runHerdr(ctx, "tab", "rename", link.tabID, label); rollbackErr != nil {
				rollbackErrors = append(rollbackErrors, rollbackErr.Error())
			}
		}
		if agentRenamed {
			if _, rollbackErr := s.runHerdr(ctx, "agent", "rename", link.paneID, link.agentName); rollbackErr != nil {
				rollbackErrors = append(rollbackErrors, rollbackErr.Error())
			}
		}
		if len(rollbackErrors) > 0 {
			return errors.Join(err, fmt.Errorf("rollback Herdr task rename: %s", strings.Join(rollbackErrors, "; ")))
		}
		return err
	}

	if link.agentName != newAgentName {
		if _, err := s.runHerdr(ctx, "agent", "rename", link.paneID, newAgentName); err != nil {
			return err
		}
		agentRenamed = true
	}
	if _, err := s.runHerdr(ctx, "tab", "rename", link.tabID, newTaskID); err != nil {
		return tryErr(err)
	}
	tabRenamed = true
	return nil
}

// registerHerdrRoutes registers herdr endpoints wired outside the generated
// oapi-codegen router (manual echoes, mirroring registerGitRoutes).
func (s *Server) registerHerdrRoutes(e *echo.Echo, basePath string) {
	wrap := func(method, path string, h http.HandlerFunc) {
		e.Add(method, basePath+path, echo.WrapHandler(h))
	}
	wrap(http.MethodPost, "/api/herdr/agents/start-file", s.StartHerdrFileAgent)
	wrap(http.MethodPost, "/api/herdr/agents/start-task", s.StartTaskAgent)
	wrap(http.MethodPost, "/api/herdr/agents/focus", s.FocusHerdrAgent)
	wrap(http.MethodGet, "/api/herdr/agent-kinds", s.ListHerdrAgentKinds)
	wrap(http.MethodGet, "/api/herdr/layouts", s.ListHerdrLayouts)
	wrap(http.MethodPost, "/api/herdr/panes/resize", s.ResizeHerdrPane)
}

// ListHerdrAgentKinds returns the agent kinds herdr can start in a pane.
func (s *Server) ListHerdrAgentKinds(w http.ResponseWriter, r *http.Request) {
	kinds := make([]string, 0, len(herdrAgentKinds))
	for k := range herdrAgentKinds {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	writeJSONResponse(w, http.StatusOK, map[string]any{"kinds": kinds})
}

// StartHerdrAgent starts a general-purpose agent in a tab belonging to the
// current project. The tab and pane are resolved server-side so that the
// client never has to coordinate Herdr's workspace topology itself.
func (s *Server) StartHerdrAgent(w http.ResponseWriter, r *http.Request) {
	var req api.HerdrAgentStartRequest
	if !decodeBody(w, r, &req) {
		return
	}
	tab := strings.TrimSpace(req.Tab)
	if _, ok := validHerdrLabel(tab); !ok {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("tab must be between 1 and 80 characters")})
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if !validHerdrAgentKind(kind) {
		writeError(w, &httpError{status: http.StatusBadRequest, err: fmt.Errorf("unsupported agent kind %q", kind)})
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}

	// Keep workspace/tab selection and generated-name allocation atomic across
	// concurrent browser requests.
	s.herdrAgentStartMu.Lock()
	defer s.herdrAgentStartMu.Unlock()

	agent, err := s.startHerdrAgent(r.Context(), app, kind, tab)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.HerdrAgentStartResponse{Ok: true, Agent: *agent})
}

func herdrGenericAgentSlug(label string) string {
	var b strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(label)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
			separator = false
		case r == '-':
			if b.Len() > 0 && !separator {
				b.WriteRune(r)
				separator = true
			}
		default:
			if b.Len() > 0 && !separator {
				b.WriteByte('-')
				separator = true
			}
		}
	}
	return strings.Trim(b.String(), "-_")
}

func herdrGenericAgentName(label string, used map[string]bool) string {
	slug := herdrGenericAgentSlug(label)
	if slug == "" {
		slug = "agent"
	}
	base := "mybox-" + slug
	if len(base) > 32 {
		base = strings.TrimRight(base[:32], "-_")
	}
	if !used[base] {
		return base
	}
	for suffix := 2; ; suffix++ {
		tail := fmt.Sprintf("-%d", suffix)
		prefix := base
		if len(prefix)+len(tail) > 32 {
			prefix = strings.TrimRight(prefix[:32-len(tail)], "-_")
		}
		candidate := prefix + tail
		if !used[candidate] {
			return candidate
		}
	}
}

func (s *Server) herdrLiveAgentNames(ctx context.Context) (map[string]bool, error) {
	out, err := s.runHerdr(ctx, "agent", "list")
	if err != nil {
		return nil, err
	}
	var env herdrEnvelope
	if err := json.Unmarshal(out, &env); err != nil {
		return nil, fmt.Errorf("unexpected herdr agent list output")
	}
	var list herdrAgentListResult
	if err := json.Unmarshal(env.Result, &list); err != nil {
		return nil, fmt.Errorf("unexpected herdr agent list result")
	}
	used := make(map[string]bool, len(list.Agents))
	for _, raw := range list.Agents {
		name := raw.Name
		if name == "" {
			name = raw.Agent
		}
		if name != "" {
			used[name] = true
		}
	}
	return used, nil
}

func (s *Server) startHerdrAgent(ctx context.Context, app *App, kind, label string) (*api.HerdrAgent, error) {
	usedNames, err := s.herdrLiveAgentNames(ctx)
	if err != nil {
		return nil, err
	}
	name := herdrGenericAgentName(label, usedNames)

	wsID, err := s.herdrProjectWorkspaceID(ctx, app.Project.Name)
	if err != nil {
		return nil, err
	}
	var paneID string
	if wsID == "" {
		var createdTabID string
		wsID, createdTabID, paneID, err = s.bootstrapHerdrWorkspaceIDs(ctx, app.Project.Path)
		if err != nil {
			return nil, err
		}
		if _, err := s.runHerdr(ctx, "tab", "rename", createdTabID, label); err != nil {
			return nil, err
		}
	} else {
		paneID, err = s.herdrAgentPaneForTab(ctx, wsID, label, app.Project.Path)
		if err != nil {
			return nil, err
		}
	}
	if paneID == "" {
		return nil, fmt.Errorf("no usable pane for herdr tab %q", label)
	}
	if err := s.startHerdrAgentOnPane(ctx, name, kind, paneID); err != nil {
		return nil, err
	}
	if agent := s.findHerdrAgent(ctx, name); agent != nil {
		return agent, nil
	}
	return &api.HerdrAgent{
		Name:        name,
		Status:      "unknown",
		WorkspaceId: wsID,
		PaneId:      paneID,
	}, nil
}

// StartHerdrFileAgent starts a herdr agent dedicated to a task directory. It
// creates a tab labelled with the task title in the project's
// workspace, starts the agent on the tab's root pane using the task-directory
// derived agent name, and returns the agent so the UI can read output / send
// prompts. When an agent for the task directory is already running (or a tab
// for it already exists) it is reused; every file inside the same task
// directory shares the agent.
func (s *Server) StartHerdrFileAgent(w http.ResponseWriter, r *http.Request) {
	var req herdrStartFileAgentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	path := strings.TrimSpace(req.Path)
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || len(path) > 1024 {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("invalid file path")})
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	agent, err := s.startHerdrFileAgent(r.Context(), app, path, strings.TrimSpace(req.Kind))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, herdrFileAgentResponse{Ok: true, Agent: agent})
}

// startHerdrFileAgent launches (or reuses) the herdr agent dedicated to a
// project-relative file inside a task directory. kind defaults to "opencode".
func (s *Server) startHerdrFileAgent(ctx context.Context, app *App, path string, kind string) (*api.HerdrAgent, error) {
	if path == "" || strings.HasPrefix(path, "/") || strings.Contains(path, "..") || len(path) > 1024 {
		return nil, &httpError{status: http.StatusBadRequest, err: errors.New("invalid file path")}
	}
	dir, ok := herdrTaskDirFromPath(path)
	if !ok {
		return nil, &httpError{status: http.StatusBadRequest, err: errors.New("agents can only be started for files inside a task directory (_tasks/<dir>/...)")}
	}
	var task *domain.Task
	if candidate, err := app.Tasks.Show(ctx, dir); err == nil {
		task = candidate
	}
	label := dir
	if task != nil && strings.TrimSpace(task.Title) != "" {
		label = task.Title
	}
	label, ok = validHerdrTaskTitle(label)
	if !ok {
		return nil, &httpError{status: http.StatusBadRequest, err: errors.New("task title must be between 1 and 80 characters and cannot contain control characters")}
	}
	name := herdrTaskAgentName(dir)
	if name == "" {
		return nil, &httpError{status: http.StatusBadRequest, err: errors.New("task directory name does not produce a valid agent name")}
	}
	kind = strings.TrimSpace(kind)
	if kind == "" {
		kind = "opencode"
	}
	if !validHerdrAgentKind(kind) {
		return nil, &httpError{status: http.StatusBadRequest, err: fmt.Errorf("unsupported agent kind %q", kind)}
	}

	// The task directory already has a live agent: report it instead of
	// stacking tabs. Every file in the directory shares the same agent.
	if agent := s.findHerdrAgent(ctx, name); agent != nil {
		attachHerdrTask(agent, task)
		return agent, nil
	}

	var wsID, paneID string
	var err error
	wsID, err = s.herdrProjectWorkspaceID(ctx, app.Project.Name)
	if err != nil {
		return nil, err
	}
	if wsID == "" {
		// No workspace carries the project label yet; bootstrap one. Its fresh
		// root tab becomes the task's tab, so it only needs the dir label.
		createdWsID, createdTabID, createdPaneID, err := s.bootstrapHerdrWorkspaceIDs(ctx, app.Project.Path)
		if err != nil {
			return nil, err
		}
		if _, err := s.runHerdr(ctx, "tab", "rename", createdTabID, label); err != nil {
			return nil, err
		}
		wsID, paneID = createdWsID, createdPaneID
	} else {
		// Reuse the task's existing tab when present, otherwise create one.
		_, paneID, err = s.herdrFileTab(ctx, wsID, label)
		if err != nil {
			return nil, err
		}
		if paneID == "" {
			paneID, err = s.herdrCreateFileTab(ctx, wsID, label, app.Project.Path)
			if err != nil {
				return nil, err
			}
		}
	}
	if paneID == "" {
		return nil, fmt.Errorf("no usable pane for task agent %q", name)
	}

	if err := s.startHerdrAgentOnPane(ctx, name, kind, paneID); err != nil {
		return nil, err
	}

	agent := s.findHerdrAgent(ctx, name)
	if agent == nil {
		agent = &api.HerdrAgent{Name: name, Status: "unknown", WorkspaceId: wsID, PaneId: paneID}
	}
	attachHerdrTask(agent, task)
	return agent, nil
}

// herdrStartTaskAgentRequest starts (or reuses) the herdr agent dedicated to a
// task file and optionally sends it a prompt immediately.
type herdrStartTaskAgentRequest struct {
	// TaskID identifies the task whose markdown file the agent works on.
	TaskID string `json:"task_id"`
	// AgentKind selects the agent executable (default: opencode).
	AgentKind string `json:"agent_kind"`
	// Prompt is either a prompt template name (prompts/<name>.md, "@"-prefixed
	// or matching an existing template) or an inline prompt. When empty no
	// prompt is sent.
	Prompt string `json:"prompt"`
}

// herdrStartTaskAgentResponse reports the agent started for a task and the
// prompt that was submitted (when one was given).
type herdrStartTaskAgentResponse struct {
	Ok    bool            `json:"ok"`
	Agent *api.HerdrAgent `json:"agent,omitempty"`
	// Prompt is the rendered prompt text sent to the agent.
	Prompt string `json:"prompt,omitempty"`
}

// StartTaskAgent starts a herdr agent dedicated to a task's markdown file. The
// tab is labelled after the task title and the agent (agent_kind) runs bound to
// that file, mirroring the file-agent flow. When a prompt is given it is
// rendered (via a prompts/<name>.md template when it matches one) and submitted
// to the agent immediately.
func (s *Server) StartTaskAgent(w http.ResponseWriter, r *http.Request) {
	var req herdrStartTaskAgentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.TaskID = strings.TrimSpace(req.TaskID)
	if req.TaskID == "" || strings.ContainsAny(req.TaskID, `/\`) {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("invalid task id")})
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	task, err := app.Tasks.Show(r.Context(), req.TaskID)
	if err != nil {
		writeError(w, err)
		return
	}
	kind := strings.TrimSpace(req.AgentKind)
	if kind == "" {
		kind = "opencode"
	}
	if !validHerdrAgentKind(kind) {
		writeError(w, &httpError{status: http.StatusBadRequest, err: fmt.Errorf("unsupported agent kind %q", kind)})
		return
	}
	relPath := app.Tasks.RelativePathFor(task)
	agent, err := s.startHerdrFileAgent(r.Context(), app, relPath, kind)
	if err != nil {
		writeError(w, err)
		return
	}
	resp := herdrStartTaskAgentResponse{Ok: true, Agent: agent}
	if prompt := strings.TrimSpace(req.Prompt); prompt != "" {
		text, err := app.Tasks.RenderPrompt(r.Context(), prompt, task)
		if err != nil {
			writeError(w, err)
			return
		}
		if strings.TrimSpace(text) != "" {
			if _, err := s.runHerdr(r.Context(), "agent", "prompt", agent.Name, text); err != nil {
				writeError(w, err)
				return
			}
			resp.Prompt = text
		}
	}
	writeJSONResponse(w, http.StatusOK, resp)
}

// bootstrapHerdrWorkspaceIDs creates herdr's first workspace for the current
// project and returns the ids of its fresh root workspace, tab and pane. It is
// the workspace bootstrap used for file agents, capturing the ids the CLI
// returns so the caller can rename the root tab and start an agent on its pane.
func (s *Server) bootstrapHerdrWorkspaceIDs(ctx context.Context, cwd string) (wsID, tabID, paneID string, err error) {
	args := []string{"workspace", "create"}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	out, err := s.runHerdr(ctx, args...)
	if err != nil {
		return "", "", "", err
	}
	wsID, tabID, paneID = herdrCreatedTabParse(out)
	if wsID == "" || tabID == "" || paneID == "" {
		return "", "", "", fmt.Errorf("herdr workspace create returned no root workspace/tab/pane")
	}
	return wsID, tabID, paneID, nil
}
