package myherdr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

var (
	workspaceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	sshHostPattern     = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.@-]{0,252}$`)
	sshArgumentPattern = regexp.MustCompile(`^[A-Za-z0-9_.:@/-]+$`)
	sshHostLinePattern = regexp.MustCompile(`^\s*-\s*` + "`" + `([^` + "`" + `]+)` + "`" + `\s*$`)
)

type RemoteSession struct {
	Name         string `toml:"-"`
	Host         string `toml:"host"`
	HerdrSession string `toml:"herdr_session"`
}

type remoteSessionsFile struct {
	Sessions map[string]RemoteSession `toml:"sessions"`
}

func (project Project) remoteSession(name string) (*RemoteSession, bool, error) {
	if err := ValidateSessionName(name); err != nil {
		return nil, false, err
	}
	path := filepath.Join(project.Root, ".myherdr", "sessions.toml")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read .myherdr/sessions.toml: %w", err)
	}
	var config remoteSessionsFile
	decoder := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return nil, false, fmt.Errorf("parse .myherdr/sessions.toml: %w", err)
	}
	remote, ok := config.Sessions[name]
	if !ok {
		return nil, false, nil
	}
	if err := ValidateSessionName(remote.HerdrSession); err != nil {
		return nil, true, fmt.Errorf("remote Herdr session for %q is invalid: %w", name, err)
	}
	if !sshHostPattern.MatchString(remote.Host) || strings.Contains(remote.Host, "*") {
		return nil, true, fmt.Errorf("remote SSH host for %q has an invalid format", name)
	}
	allowedHosts, err := readAllowedSSHHosts(filepath.Join(project.Root, ".myherdr", "ssh-hosts.md"))
	if err != nil {
		return nil, true, err
	}
	if _, allowed := allowedHosts[remote.Host]; !allowed {
		return nil, true, fmt.Errorf("remote SSH host %q is not listed in .myherdr/ssh-hosts.md", remote.Host)
	}
	remote.Name = name
	return &remote, true, nil
}

func readAllowedSSHHosts(path string) (map[string]struct{}, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]struct{}{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read .myherdr/ssh-hosts.md: %w", err)
	}
	allowed := make(map[string]struct{})
	for _, line := range strings.Split(string(data), "\n") {
		match := sshHostLinePattern.FindStringSubmatch(strings.TrimSuffix(line, "\r"))
		if len(match) != 2 {
			continue
		}
		host := match[1]
		if !sshHostPattern.MatchString(host) || strings.Contains(host, "*") {
			return nil, fmt.Errorf("invalid SSH host %q in .myherdr/ssh-hosts.md; list explicit host names only", host)
		}
		allowed[host] = struct{}{}
	}
	return allowed, nil
}

func listRemotePanes(ctx context.Context, remote *RemoteSession) ([]Pane, error) {
	workspaceData, err := runRemoteHerdrJSON(ctx, remote, "workspace", "list")
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
		data, err := runRemoteHerdrJSON(ctx, remote, "pane", "list", "--workspace", workspaceID)
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

func runRemoteHerdrJSON(ctx context.Context, remote *RemoteSession, args ...string) (any, error) {
	for _, arg := range args {
		if !sshArgumentPattern.MatchString(arg) {
			return nil, errors.New("refusing an unsafe argument for remote Herdr")
		}
	}
	ssh, err := exec.LookPath("ssh")
	if err != nil {
		return nil, errors.New("OpenSSH client is not installed or not on PATH")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	commandArgs := []string{"-T", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2", "--", remote.Host, "env", "HERDR_SESSION=" + remote.HerdrSession, "herdr"}
	commandArgs = append(commandArgs, args...)
	command := exec.CommandContext(ctx, ssh, commandArgs...)
	var stdout bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("cannot query remote Herdr session %q on %q; check non-interactive SSH access and the remote Herdr server", remote.HerdrSession, remote.Host)
	}
	var data any
	if err := json.Unmarshal(stdout.Bytes(), &data); err != nil {
		return nil, errors.New("remote Herdr returned an unreadable pane list")
	}
	return data, nil
}
