//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package myherdr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteSessionRequiresAnExplicitlyAllowedHost(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, ".myherdr")
	if err := os.Mkdir(configDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "sessions.toml"), []byte("[sessions.remote-codex]\nhost = \"devbox\"\nherdr_session = \"codex-project\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(configDir, "ssh-hosts.md")
	if err := os.WriteFile(policyPath, []byte("# allowed\n- `devbox`\n"), 0600); err != nil {
		t.Fatal(err)
	}

	project := Project{Root: root}
	remote, isRemote, err := project.remoteSession("remote-codex")
	if err != nil {
		t.Fatal(err)
	}
	if !isRemote || remote.Host != "devbox" || remote.HerdrSession != "codex-project" {
		t.Fatalf("remoteSession() = %#v, %v; want allowlisted remote", remote, isRemote)
	}

	if err := os.WriteFile(policyPath, []byte("# empty policy\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, isRemote, err := project.remoteSession("remote-codex"); err == nil || !isRemote {
		t.Fatalf("remoteSession() = remote=%v, err=%v; want missing allowlist error", isRemote, err)
	}
}

func TestRemotePaneListUsesFakeSSHAndHerdrExecutables(t *testing.T) {
	binDir := t.TempDir()
	capturePath := filepath.Join(t.TempDir(), "ssh-args")
	writeFakeExecutable(t, binDir, "ssh", `
printf '%s\n' "$*" >> "$MYHERDR_TEST_CAPTURE"
case " $* " in
  *" workspace list "*) printf '%s\n' '{"result":{"workspaces":[{"workspace_id":"w1"}]}}' ;;
  *" pane list "*) printf '%s\n' '{"result":{"panes":[{"pane_id":"w1:p1","label":"Codex","agent":"codex","cwd":"/srv/project"}]}}' ;;
  *) exit 2 ;;
esac
`)
	prependTestPath(t, binDir)
	t.Setenv("MYHERDR_TEST_CAPTURE", capturePath)
	remote := &RemoteSession{Host: "devbox", HerdrSession: "codex-project"}
	panes, err := listRemotePanes(context.Background(), remote)
	if err != nil {
		t.Fatal(err)
	}
	if len(panes) != 1 || panes[0].ID != "w1:p1" || !panes[0].IsCodex {
		t.Fatalf("listRemotePanes() = %#v, want one Codex pane", panes)
	}

	args, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"-- devbox env HERDR_SESSION=codex-project herdr workspace list", "-- devbox env HERDR_SESSION=codex-project herdr pane list --workspace w1"} {
		if !strings.Contains(string(args), expected) {
			t.Errorf("fake ssh did not receive %q; args were %q", expected, args)
		}
	}
}
