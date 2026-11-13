package myherdr

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultSessionNameSlugsProjectName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{name: "My Project!!", want: "codex-my-project"},
		{name: "__alpha--beta__", want: "codex-alpha-beta"},
		{name: "日本語", want: "codex-project"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DefaultSessionName(test.name); got != test.want {
				t.Fatalf("DefaultSessionName(%q) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}

func TestDefaultSessionNameFitsHerdrLimitForLongProjectNames(t *testing.T) {
	projectName := strings.Repeat("a", 59)
	want := "codex-" + strings.Repeat("a", 58)
	if got := DefaultSessionName(projectName); got != want {
		t.Fatalf("DefaultSessionName(long name) = %q, want %q", got, want)
	}
}

func TestValidateSessionNameMatchesHerdrPunctuationRules(t *testing.T) {
	for _, name := range []string{"_work", "-work", "_", "-"} {
		t.Run(name, func(t *testing.T) {
			if err := ValidateSessionName(name); err != nil {
				t.Fatalf("ValidateSessionName(%q) = %v, want success", name, err)
			}
		})
	}
	for _, name := range []string{"", ".", "..", "bad/name"} {
		t.Run("reject_"+name, func(t *testing.T) {
			if err := ValidateSessionName(name); err == nil {
				t.Fatalf("ValidateSessionName(%q) succeeded, want error", name)
			}
		})
	}
	if err := ValidateSessionName(strings.Repeat("a", 64)); err != nil {
		t.Fatalf("ValidateSessionName(64 characters) = %v, want success", err)
	}
	if err := ValidateSessionName(strings.Repeat("a", 65)); err == nil {
		t.Fatal("ValidateSessionName(65 characters) succeeded, want error")
	}
}

func TestSessionURLIncludesPaneQuery(t *testing.T) {
	got, err := SessionURL(12345, "codex-my-project", "w1:p2")
	if err != nil {
		t.Fatal(err)
	}
	want := "http://127.0.0.1:12345/sessions/codex-my-project?pane=w1%3Ap2"
	if got != want {
		t.Fatalf("SessionURL() = %q, want %q", got, want)
	}
}

func TestSessionURLRejectsInvalidNamesAndPaneIDs(t *testing.T) {
	if _, err := SessionURL(12345, "../codex", ""); err == nil {
		t.Fatal("expected invalid session name to be rejected")
	}
	if _, err := SessionURL(12345, "codex-project", "w1:p1;touch"); err == nil {
		t.Fatal("expected invalid pane ID to be rejected")
	}
}

func TestResolveProjectUsesGitRootAndNonRepositoryDirectory(t *testing.T) {
	temp := t.TempDir()
	repository := filepath.Join(temp, "repository")
	subdirectory := filepath.Join(repository, "nested")
	if err := os.MkdirAll(subdirectory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "init", "--quiet", repository).Run(); err != nil {
		t.Fatal(err)
	}

	project, err := ResolveProject(subdirectory)
	if err != nil {
		t.Fatal(err)
	}
	if project.Root != repository || project.Name != "repository" {
		t.Fatalf("ResolveProject() = %#v, want repository root %q", project, repository)
	}

	nonRepository := filepath.Join(temp, "standalone")
	if err := os.Mkdir(nonRepository, 0700); err != nil {
		t.Fatal(err)
	}
	project, err = ResolveProject(nonRepository)
	if err != nil {
		t.Fatal(err)
	}
	if project.Root != nonRepository || project.Name != "standalone" {
		t.Fatalf("ResolveProject() = %#v, want standalone directory %q", project, nonRepository)
	}
}

func TestSessionNameCommandPrintsExactHerdrCommand(t *testing.T) {
	projectDir := t.TempDir()
	var output strings.Builder
	if err := Run(context.Background(), []string{"session-name", "--project", projectDir, "--session", "codex-explicit"}, strings.NewReader(""), &output, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "herdr --session codex-explicit\n"; got != want {
		t.Fatalf("session-name output = %q, want %q", got, want)
	}
}
