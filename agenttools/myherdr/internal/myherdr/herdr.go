package myherdr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type Pane struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	CWD     string `json:"cwd,omitempty"`
	IsCodex bool   `json:"is_codex"`
}

type terminalTarget struct {
	Session string
	Pane    string
	Remote  *RemoteSession
}

func listPanes(ctx context.Context, project Project, session string) ([]Pane, error) {
	if err := ValidateSessionName(session); err != nil {
		return nil, err
	}
	if remote, isRemote, err := project.remoteSession(session); err != nil {
		return nil, err
	} else if isRemote {
		return listRemotePanes(ctx, remote)
	}
	return listLocalPanes(ctx, session)
}

func listLocalPanes(ctx context.Context, session string) ([]Pane, error) {
	workspaceData, err := runHerdrJSON(ctx, session, "workspace", "list")
	if err != nil {
		return nil, err
	}
	workspaceIDs := collectStringFields(workspaceData, "workspace_id")
	sort.Strings(workspaceIDs)
	var panes []Pane
	seen := make(map[string]struct{})
	for _, workspaceID := range workspaceIDs {
		if !workspaceIDPattern.MatchString(workspaceID) {
			continue
		}
		data, err := runHerdrJSON(ctx, session, "pane", "list", "--workspace", workspaceID)
		if err != nil {
			return nil, err
		}
		for _, pane := range parsePaneList(data) {
			if _, duplicate := seen[pane.ID]; duplicate {
				continue
			}
			seen[pane.ID] = struct{}{}
			panes = append(panes, pane)
		}
	}
	sort.Slice(panes, func(i, j int) bool { return panes[i].ID < panes[j].ID })
	return panes, nil
}

func runHerdrJSON(ctx context.Context, session string, args ...string) (any, error) {
	binary, err := exec.LookPath("herdr")
	if err != nil {
		return nil, errors.New("herdr is not installed or not on PATH; install it from https://herdr.dev/")
	}
	commandArgs := append([]string{"--session", session}, args...)
	command := exec.CommandContext(ctx, binary, commandArgs...)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("cannot query Herdr session %q; ensure it is running with `%s`", session, SessionCommand(session))
	}
	var data any
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		return nil, errors.New("herdr returned an unreadable pane list")
	}
	return data, nil
}

func parsePaneList(data any) []Pane {
	byID := make(map[string]Pane)
	visitObjects(data, func(object map[string]any) {
		id, ok := object["pane_id"].(string)
		if !ok || ValidatePaneID(id) != nil {
			return
		}
		label := firstString(object, "label", "title", "name")
		cwd := firstString(object, "cwd", "path")
		if label == "" {
			label = id
		}
		pane := byID[id]
		pane.ID = id
		if pane.Label == "" || pane.Label == id && label != id {
			pane.Label = label
		}
		if pane.CWD == "" {
			pane.CWD = cwd
		}
		pane.IsCodex = pane.IsCodex || objectHasCodexAgent(object)
		byID[id] = pane
	})
	panes := make([]Pane, 0, len(byID))
	for _, pane := range byID {
		panes = append(panes, pane)
	}
	return panes
}

func firstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func objectHasCodexAgent(object map[string]any) bool {
	for _, key := range []string{"agent", "agent_kind", "agent_name", "kind"} {
		if value, ok := object[key]; ok && valueContainsCodex(value) {
			return true
		}
	}
	return false
}

func valueContainsCodex(value any) bool {
	switch typed := value.(type) {
	case string:
		return strings.EqualFold(typed, "codex")
	case map[string]any:
		for key, nested := range typed {
			if key == "agent" || key == "kind" || key == "name" || key == "agent_kind" {
				if valueContainsCodex(nested) {
					return true
				}
			}
		}
	}
	return false
}

func collectStringFields(data any, field string) []string {
	var values []string
	seen := make(map[string]struct{})
	visitObjects(data, func(object map[string]any) {
		if value, ok := object[field].(string); ok && value != "" {
			if _, exists := seen[value]; exists {
				return
			}
			seen[value] = struct{}{}
			values = append(values, value)
		}
	})
	return values
}

func visitObjects(value any, visit func(map[string]any)) {
	switch typed := value.(type) {
	case map[string]any:
		visit(typed)
		for _, child := range typed {
			visitObjects(child, visit)
		}
	case []any:
		for _, child := range typed {
			visitObjects(child, visit)
		}
	}
}

func herdrEnvironment(session string) []string {
	environment := make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "HERDR_SESSION=") || strings.HasPrefix(item, "HERDR_SOCKET_PATH=") {
			continue
		}
		environment = append(environment, item)
	}
	return append(environment, "HERDR_SESSION="+session)
}
