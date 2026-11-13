//go:build linux || darwin || freebsd || openbsd || netbsd || dragonfly

package myherdr

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartInvokesHerdrWithProjectRootAndDerivedSession(t *testing.T) {
	t.Setenv("HERDR_ENV", "")
	projectDir := filepath.Join(t.TempDir(), "My Project")
	if err := os.Mkdir(projectDir, 0700); err != nil {
		t.Fatal(err)
	}
	binDir := t.TempDir()
	capturePath := filepath.Join(t.TempDir(), "herdr-call")
	writeFakeExecutable(t, binDir, "herdr", `
printf 'cwd=%s\nargs=%s\n' "$PWD" "$*" > "$MYHERDR_TEST_CAPTURE"
`)
	prependTestPath(t, binDir)
	t.Setenv("MYHERDR_TEST_CAPTURE", capturePath)
	var output strings.Builder
	if err := Run(context.Background(), []string{"start", "--project", projectDir}, strings.NewReader(""), &output, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	captured, err := os.ReadFile(capturePath)
	if err != nil {
		t.Fatal(err)
	}
	want := "cwd=" + projectDir + "\nargs=--session codex-my-project\n"
	if string(captured) != want {
		t.Fatalf("fake Herdr received %q, want %q", captured, want)
	}
}

func TestStartInsideHerdrDoesNotLaunchNestedSession(t *testing.T) {
	emptyPath := t.TempDir()
	projectDir := t.TempDir()
	t.Setenv("PATH", emptyPath)
	t.Setenv("HERDR_ENV", "1")
	t.Setenv("HERDR_SESSION", "existing-session")
	var output strings.Builder
	if err := Run(context.Background(), []string{"start", "--project", projectDir}, strings.NewReader(""), &output, &strings.Builder{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `Already inside Herdr session "existing-session"`) {
		t.Fatalf("nested-session message = %q", output.String())
	}
}
