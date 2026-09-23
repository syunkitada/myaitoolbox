package mygit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncInitializesMissingLockfile(t *testing.T) {
	remote := createRemoteRepository(t)
	root := t.TempDir()
	manifest := fmt.Sprintf("repositories:\n  - name: example\n    url: %s\n    revision: main\n", remote)
	writeFile(t, filepath.Join(root, ManifestFilename), manifest)

	service := NewService()
	var output bytes.Buffer
	if err := service.SyncWithOutput(context.Background(), root, &output); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	for _, action := range []string{"sync: workspace", "sync: clone example", "sync: checkout example", "sync: write lockfile"} {
		if !strings.Contains(output.String(), action) {
			t.Fatalf("sync output does not contain %q: %s", action, output.String())
		}
	}

	lock, err := LoadLockfile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatalf("LoadLockfile() error = %v", err)
	}
	lockData, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(lockData), "version:") {
		t.Fatalf("generated lockfile contains removed version field: %s", lockData)
	}
	if len(lock.Repositories) != 1 || !isObjectID(lock.Repositories[0].Commit) {
		t.Fatalf("unexpected lockfile: %#v", lock)
	}
	target := filepath.Join(root, DefaultRepoDir, "example")
	head := gitOutput(t, target, "rev-parse", "HEAD")
	if strings.TrimSpace(head) != lock.Repositories[0].Commit {
		t.Fatalf("initial sync checked out %s, want %s", strings.TrimSpace(head), lock.Repositories[0].Commit)
	}

	appendRemoteCommit(t, remote)
	if err := service.Sync(context.Background(), root); err != nil {
		t.Fatalf("Sync() with an existing lockfile error = %v", err)
	}
	lockAfter, err := LoadLockfile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatalf("LoadLockfile() after second sync error = %v", err)
	}
	if lockAfter.Repositories[0].Commit != lock.Repositories[0].Commit {
		t.Fatalf("second sync changed lockfile commit to %s, want %s", lockAfter.Repositories[0].Commit, lock.Repositories[0].Commit)
	}
}

func TestSyncResolvesRepositoriesMissingFromLockfile(t *testing.T) {
	firstRemote := createRemoteRepository(t)
	secondRemote := createRemoteRepository(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ManifestFilename), fmt.Sprintf("repositories:\n  - name: first\n    url: %s\n    revision: main\n", firstRemote))

	service := NewService()
	if err := service.Update(context.Background(), root); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	writeFile(t, filepath.Join(root, ManifestFilename), fmt.Sprintf("repositories:\n  - name: first\n    url: %s\n    revision: main\n  - name: second\n    url: %s\n    revision: main\n", firstRemote, secondRemote))

	var output bytes.Buffer
	if err := service.SyncWithOutput(context.Background(), root, &output); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if !strings.Contains(output.String(), "sync: resolve missing second") {
		t.Fatalf("sync output does not mention missing repository: %s", output.String())
	}
	lock, err := LoadLockfile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatalf("LoadLockfile() error = %v", err)
	}
	if len(lock.Repositories) != 2 {
		t.Fatalf("lockfile contains %d repositories, want 2", len(lock.Repositories))
	}
	if _, err := os.Stat(filepath.Join(root, DefaultRepoDir, "second")); err != nil {
		t.Fatalf("missing repository was not cloned: %v", err)
	}
}

func TestUpdateSyncAndStatusWithLocalRemote(t *testing.T) {
	remote := createRemoteRepository(t)
	root := t.TempDir()
	manifest := fmt.Sprintf("repositories:\n  - name: example\n    url: %s\n    revision: main\n", remote)
	writeFile(t, filepath.Join(root, ManifestFilename), manifest)

	service := NewService()
	ctx := context.Background()
	var updateOutput bytes.Buffer
	if err := service.UpdateWithOutput(ctx, root, &updateOutput); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	for _, action := range []string{"update: workspace", "update: resolve example", "update: checkout example", "update: write lockfile"} {
		if !strings.Contains(updateOutput.String(), action) {
			t.Fatalf("update output does not contain %q: %s", action, updateOutput.String())
		}
	}
	lock, err := LoadLockfile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatal(err)
	}
	if len(lock.Repositories) != 1 || !isObjectID(lock.Repositories[0].Commit) {
		t.Fatalf("unexpected lockfile: %#v", lock)
	}
	target := filepath.Join(root, DefaultRepoDir, "example")
	if _, err := os.Stat(filepath.Join(target, ".git")); err != nil {
		t.Fatalf("clone was not created: %v", err)
	}
	gitignore, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gitignore), "/_repos/") {
		t.Fatalf("managed _repos block missing: %s", gitignore)
	}

	var status bytes.Buffer
	if err := service.Status(ctx, root, &status); err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !strings.Contains(status.String(), "status:   locked") {
		t.Fatalf("unexpected status output: %s", status.String())
	}

	appendRemoteCommit(t, remote)
	status.Reset()
	if err := service.Status(ctx, root, &status); err != nil {
		t.Fatalf("Status() after remote update error = %v", err)
	}
	if !strings.Contains(status.String(), "status:   remote-outdated") {
		t.Fatalf("remote update was not reported: %s", status.String())
	}
	oldCommit := lock.Repositories[0].Commit
	if err := service.Sync(ctx, root); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	head := gitOutput(t, target, "rev-parse", "HEAD")
	if strings.TrimSpace(head) != oldCommit {
		t.Fatalf("Sync() changed locked commit to %s, want %s", strings.TrimSpace(head), oldCommit)
	}

	writeFile(t, filepath.Join(target, "untracked.txt"), "do not remove")
	if err := service.Sync(ctx, root); err == nil {
		t.Fatal("Sync() accepted a dirty repository")
	}
	if _, err := os.Stat(filepath.Join(target, "untracked.txt")); err != nil {
		t.Fatalf("dirty file was removed: %v", err)
	}
}

func TestStatusAndUpdateProtectIgnoredUntrackedFiles(t *testing.T) {
	remote := createRemoteRepositoryWithIgnoredPath(t)
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ManifestFilename), fmt.Sprintf("repositories:\n  - name: example\n    url: %s\n    revision: main\n", remote))

	service := NewService()
	ctx := context.Background()
	if err := service.Update(ctx, root); err != nil {
		t.Fatalf("initial Update() error = %v", err)
	}

	target := filepath.Join(root, DefaultRepoDir, "example")
	ignoredPath := filepath.Join(target, "generated.txt")
	writeFile(t, ignoredPath, "local data\n")

	var status bytes.Buffer
	if err := service.Status(ctx, root, &status); err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !strings.Contains(status.String(), "status:   dirty") {
		t.Fatalf("ignored untracked file was not reported as dirty: %s", status.String())
	}

	appendRemoteIgnoredCommit(t, remote)
	if err := service.Update(ctx, root); err == nil {
		t.Fatal("Update() accepted an ignored untracked file")
	}
	if got := string(readFile(t, ignoredPath)); got != "local data\n" {
		t.Fatalf("ignored untracked file was changed to %q", got)
	}
}

func TestResolveRevisionRejectsAmbiguousBranchAndTag(t *testing.T) {
	remote := createRemoteRepository(t)
	work := filepath.Join(t.TempDir(), "work")
	gitCommand(t, filepath.Dir(work), "clone", "-b", "main", remote, work)
	gitCommand(t, work, "config", "user.email", "test@example.test")
	gitCommand(t, work, "config", "user.name", "mygit test")
	gitCommand(t, work, "tag", "main")
	gitCommand(t, work, "push", "origin", "refs/tags/main")

	if _, err := ResolveRevision(context.Background(), CommandGit{}, remote, "main"); err == nil {
		t.Fatal("ambiguous branch and tag revision was accepted")
	}
}

func createRemoteRepository(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	gitCommand(t, base, "init", "--bare", remote)
	work := filepath.Join(base, "work")
	gitCommand(t, base, "init", "-b", "main", work)
	gitCommand(t, work, "config", "user.email", "test@example.test")
	gitCommand(t, work, "config", "user.name", "mygit test")
	writeFile(t, filepath.Join(work, "README.md"), "first\n")
	gitCommand(t, work, "add", "README.md")
	gitCommand(t, work, "commit", "-m", "first")
	gitCommand(t, work, "remote", "add", "origin", remote)
	gitCommand(t, work, "push", "origin", "main")
	return remote
}

func createRemoteRepositoryWithIgnoredPath(t *testing.T) string {
	t.Helper()
	remote := createRemoteRepository(t)
	work := filepath.Join(t.TempDir(), "work")
	gitCommand(t, filepath.Dir(work), "clone", "-b", "main", remote, work)
	gitCommand(t, work, "config", "user.email", "test@example.test")
	gitCommand(t, work, "config", "user.name", "mygit test")
	writeFile(t, filepath.Join(work, ".gitignore"), "generated.txt\n")
	gitCommand(t, work, "add", ".gitignore")
	gitCommand(t, work, "commit", "-m", "ignore generated files")
	gitCommand(t, work, "push", "origin", "main")
	return remote
}

func appendRemoteCommit(t *testing.T, remote string) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	gitCommand(t, filepath.Dir(work), "clone", "-b", "main", remote, work)
	gitCommand(t, work, "config", "user.email", "test@example.test")
	gitCommand(t, work, "config", "user.name", "mygit test")
	writeFile(t, filepath.Join(work, "second.txt"), "second\n")
	gitCommand(t, work, "add", "second.txt")
	gitCommand(t, work, "commit", "-m", "second")
	gitCommand(t, work, "push", "origin", "main")
}

func appendRemoteIgnoredCommit(t *testing.T, remote string) {
	t.Helper()
	work := filepath.Join(t.TempDir(), "work")
	gitCommand(t, filepath.Dir(work), "clone", "-b", "main", remote, work)
	gitCommand(t, work, "config", "user.email", "test@example.test")
	gitCommand(t, work, "config", "user.name", "mygit test")
	writeFile(t, filepath.Join(work, "generated.txt"), "remote data\n")
	gitCommand(t, work, "add", "-f", "generated.txt")
	gitCommand(t, work, "commit", "-m", "track generated file")
	gitCommand(t, work, "push", "origin", "main")
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", args...)
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	return string(output)
}

func gitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	_ = gitOutput(t, dir, args...)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
