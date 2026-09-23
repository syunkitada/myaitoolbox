package entrypoint

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type apiGitDetail struct {
	IsRepo            bool       `json:"is_repo"`
	Branch            string     `json:"branch"`
	Remote            string     `json:"remote"`
	Upstream          string     `json:"upstream"`
	SyncStatus        string     `json:"sync_status"`
	Ahead             int        `json:"ahead"`
	Behind            int        `json:"behind"`
	LastCommitMessage string     `json:"last_commit_message"`
	Staged            []*gitFile `json:"staged"`
	Unstaged          []*gitFile `json:"unstaged"`
	Untracked         []*gitFile `json:"untracked"`
}

type apiGitResult struct {
	Ok     bool   `json:"ok"`
	Output string `json:"output"`
}

func runGitErr(dir string, args ...string) error {
	_, err := runGit(dir, args...)
	return err
}

func TestGitStatusNotARepo(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	detail := decode[apiGitDetail](t, rec)
	assert.False(t, detail.IsRepo)
}

func TestGitWorktreeIsDetectedAsRepo(t *testing.T) {
	dir := t.TempDir()
	worktree := filepath.Join(t.TempDir(), "worktree")
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("root\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "README.md"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "initial"))
	require.NoError(t, runGitErr(dir, "worktree", "add", "-b", "worktree", worktree))

	assert.True(t, isGitDir(worktree))
}

func TestGitCommitCycle(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello\n"), 0o644))

	// Untracked file appears in the status response.
	rec := do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	detail := decode[apiGitDetail](t, rec)
	require.True(t, detail.IsRepo)
	assert.Equal(t, "master", detail.Branch)
	require.Len(t, detail.Untracked, 1)
	assert.Equal(t, "a.md", detail.Untracked[0].Path)
	assert.Contains(t, detail.Untracked[0].Diff, "hello")

	// Staging moves the file between sections and exposes its diff.
	rec = do(t, s, http.MethodPost, "/api/git/stage", map[string]any{"paths": []string{"a.md"}}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	require.Len(t, detail.Staged, 1)
	assert.Equal(t, "a.md", detail.Staged[0].Path)
	assert.Len(t, detail.Untracked, 0)

	// Unstaging flips it back.
	rec = do(t, s, http.MethodPost, "/api/git/unstage", map[string]any{"paths": []string{"a.md"}}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	assert.Len(t, detail.Staged, 0)
	require.Len(t, detail.Untracked, 1)

	// Committing everything captures the file.
	rec = do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": "add a"}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	assert.Len(t, detail.Staged, 0)
	assert.Len(t, detail.Unstaged, 0)
	assert.Len(t, detail.Untracked, 0)

	// Empty commit message is rejected.
	rec = do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": " "}, "X-Project", "test")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Modified file shows up as unstaged with a diff.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello world\n"), 0o644))
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	require.Len(t, detail.Unstaged, 1)
	assert.Equal(t, "a.md", detail.Unstaged[0].Path)
	assert.Contains(t, detail.Unstaged[0].Diff, "-hello")

	// Discarding reverts the modification.
	rec = do(t, s, http.MethodPost, "/api/git/discard", map[string]any{"paths": []string{"a.md"}}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	data, err := os.ReadFile(filepath.Join(dir, "a.md"))
	require.NoError(t, err)
	assert.Equal(t, "hello\n", string(data))

	// Discarding an untracked file deletes it.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("x"), 0o644))
	rec = do(t, s, http.MethodPost, "/api/git/discard", map[string]any{"paths": []string{"untracked.txt"}}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	_, statErr := os.Stat(filepath.Join(dir, "untracked.txt"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)

	// Push without a remote reports a failure through the git result body.
	rec = do(t, s, http.MethodPost, "/api/git/push", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decode[apiGitResult](t, rec)
	assert.False(t, result.Ok)
}

func TestGitRemoteSyncStatusAndFetch(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	remote := filepath.Join(t.TempDir(), "origin.git")
	peerRoot := t.TempDir()
	peer := filepath.Join(peerRoot, "peer")

	require.NoError(t, os.MkdirAll(remote, 0o755))
	require.NoError(t, runGitErr(remote, "init", "--bare"))
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, runGitErr(dir, "remote", "add", "origin", remote))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("one\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "a.md"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "initial"))
	branch, err := runGit(dir, "branch", "--show-current")
	require.NoError(t, err)
	require.NoError(t, runGitErr(dir, "push", "-u", "origin", branch))

	rec := do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail := decode[apiGitDetail](t, rec)
	assert.Equal(t, "origin/"+branch, detail.Upstream)
	assert.Equal(t, "up-to-date", detail.SyncStatus)

	// A separate checkout advances the remote. The local remote-tracking ref
	// remains stale until the explicit fetch operation is requested.
	require.NoError(t, runGitErr(peerRoot, "clone", remote, peer))
	require.NoError(t, runGitErr(peer, "config", "user.email", "peer@example.com"))
	require.NoError(t, runGitErr(peer, "config", "user.name", "peer"))
	require.NoError(t, os.WriteFile(filepath.Join(peer, "b.md"), []byte("two\n"), 0o644))
	require.NoError(t, runGitErr(peer, "add", "b.md"))
	require.NoError(t, runGitErr(peer, "commit", "-m", "remote change"))
	require.NoError(t, runGitErr(peer, "push"))

	rec = do(t, s, http.MethodPost, "/api/git/fetch", nil, "X-Project", "test")
	result := decode[apiGitResult](t, rec)
	require.True(t, result.Ok, result.Output)
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	assert.Equal(t, 1, detail.Behind)
	assert.Equal(t, "pull", detail.SyncStatus)

	// A local commit made before pulling creates a diverged history.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "c.md"), []byte("three\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "c.md"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "local change"))
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	assert.Equal(t, 1, detail.Ahead)
	assert.Equal(t, 1, detail.Behind)
	assert.Equal(t, "diverged", detail.SyncStatus)
}

func TestGitCommitFallsBackToLocalIdentity(t *testing.T) {
	// Hide any ambient identity so the first commit would normally fail with
	// "Please tell me who you are"; the handler must fall back to a local
	// placeholder identity instead of surfacing the failure.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello\n"), 0o644))

	rec := do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": "first"}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decode[apiGitResult](t, rec)
	assert.True(t, result.Ok, "commit should succeed with the fallback identity")

	author, err := runGit(dir, "log", "-1", "--format=%an <%ae>")
	require.NoError(t, err)
	assert.Equal(t, "mybox <mybox@local>", author)
}

func TestGitAmend(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello\n"), 0o644))

	rec := do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": "first"}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)

	// The last commit subject is surfaced in the status detail.
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail := decode[apiGitDetail](t, rec)
	assert.Equal(t, "first", detail.LastCommitMessage)

	// A normal commit adds a second entry on top.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello again\n"), 0o644))
	rec = do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": "second"}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	log, err := runGit(dir, "log", "--format=%s")
	require.NoError(t, err)
	assert.Equal(t, []string{"second", "first"}, nonEmptyLines(log))

	// Amending with a message rewrites the top commit.
	rec = do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": "second amended", "amend": true}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	log, err = runGit(dir, "log", "--format=%s")
	require.NoError(t, err)
	assert.Equal(t, []string{"second amended", "first"}, nonEmptyLines(log))

	// Amending with an empty message keeps the existing message.
	rec = do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": "", "amend": true}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	assert.Equal(t, "second amended", detail.LastCommitMessage)

	// Amending on a repository without commits reports a failure through the
	// git result body instead of crashing.
	other := t.TempDir()
	require.NoError(t, runGitErr(other, "init"))
	require.NoError(t, os.WriteFile(filepath.Join(other, "x.txt"), []byte("x"), 0o644))
	s2, _ := newTestServer(t)
	s2.apps["test"].Project.Path = other
	rec = do(t, s2, http.MethodPost, "/api/git/commit", map[string]any{"message": "one", "amend": true}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	result := decode[apiGitResult](t, rec)
	assert.False(t, result.Ok)
	assert.Contains(t, result.Output, "You have nothing to amend")
}

func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func TestGitInit(t *testing.T) {
	s, app := newTestServer(t)
	rec := do(t, s, http.MethodPost, "/api/git/init", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail := decode[apiGitDetail](t, rec)
	assert.True(t, detail.IsRepo)
	assert.NotEmpty(t, app.Project.Path)
	assert.DirExists(t, filepath.Join(app.Project.Path, ".git"))
}

type apiGitLogResult struct {
	Commits []*gitLogEntry `json:"commits"`
}

type apiGitCommitDiffResult struct {
	Diff string `json:"diff"`
}

func TestGitLog(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("one\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "first commit"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("two\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "second commit"))

	rec := do(t, s, http.MethodGet, "/api/git/log", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	log := decode[apiGitLogResult](t, rec)
	require.Len(t, log.Commits, 2)
	assert.Equal(t, "second commit", log.Commits[0].Subject)
	assert.Equal(t, "first commit", log.Commits[1].Subject)
	assert.Len(t, log.Commits[0].Hash, 40)
	assert.Equal(t, log.Commits[0].Hash[:7], log.Commits[0].ShortHash)
	assert.Equal(t, "test", log.Commits[0].Author)
	assert.NotEmpty(t, log.Commits[0].Date)

	// Offset + count pagination behaves as expected when the window exceeds
	// the available commits.
	rec = do(t, s, http.MethodGet, "/api/git/log?offset=1&count=10", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	log = decode[apiGitLogResult](t, rec)
	require.Len(t, log.Commits, 1)
	assert.Equal(t, "first commit", log.Commits[0].Subject)
}

func TestGitLogNotARepo(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/git/log", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	log := decode[apiGitLogResult](t, rec)
	assert.Empty(t, log.Commits)
}

func TestGitCommitDiff(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "first"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("world\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "second"))

	hashes, err := runGit(dir, "log", "--format=%H")
	require.NoError(t, err)
	lines := nonEmptyLines(hashes)
	require.Len(t, lines, 2)

	// The root commit is diffed with git show.
	rec := do(t, s, http.MethodGet, "/api/git/diff?ref="+lines[1], nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	rootDiff := decode[apiGitCommitDiffResult](t, rec)
	assert.Contains(t, rootDiff.Diff, "a.md")
	assert.Contains(t, rootDiff.Diff, "+hello")

	// A regular commit diffs against its parent.
	rec = do(t, s, http.MethodGet, "/api/git/diff?ref="+lines[0], nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	diff := decode[apiGitCommitDiffResult](t, rec)
	assert.Contains(t, diff.Diff, "b.md")
	assert.Contains(t, diff.Diff, "+world")

	// Missing ref is rejected.
	rec = do(t, s, http.MethodGet, "/api/git/diff", nil, "X-Project", "test")
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

type apiGitBranch struct {
	Name     string `json:"name"`
	Current  bool   `json:"current"`
	Upstream string `json:"upstream"`
}

type apiGitBranchesResult struct {
	Branches []apiGitBranch `json:"branches"`
}

func TestGitBranchesAndCheckout(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "initial"))

	// Branch listing shows only master/main as current.
	rec := do(t, s, http.MethodGet, "/api/git/branches", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	branches := decode[apiGitBranchesResult](t, rec)
	require.Len(t, branches.Branches, 1)
	assert.Equal(t, "master", branches.Branches[0].Name)
	assert.True(t, branches.Branches[0].Current)

	// Checkout a new branch succeeds and it becomes current.
	rec = do(t, s, http.MethodPost, "/api/git/checkout", map[string]any{"branch": "feat", "create": true}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	res := decode[apiGitResult](t, rec)
	assert.True(t, res.Ok)

	rec = do(t, s, http.MethodGet, "/api/git/branches", nil, "X-Project", "test")
	branches = decode[apiGitBranchesResult](t, rec)
	names := []string{}
	var currentName string
	for _, b := range branches.Branches {
		if b.Current {
			currentName = b.Name
		}
		names = append(names, b.Name)
	}
	assert.Contains(t, names, "feat")
	assert.Equal(t, "feat", currentName)

	// Switching back to master works.
	rec = do(t, s, http.MethodPost, "/api/git/checkout", map[string]any{"branch": "master"}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	res = decode[apiGitResult](t, rec)
	assert.True(t, res.Ok)

	rec = do(t, s, http.MethodGet, "/api/git/branches", nil, "X-Project", "test")
	branches = decode[apiGitBranchesResult](t, rec)
	for _, b := range branches.Branches {
		if b.Current {
			currentName = b.Name
		}
	}
	assert.Equal(t, "master", currentName)

	// Checkout a nonexistent branch reports failure (not create).
	rec = do(t, s, http.MethodPost, "/api/git/checkout", map[string]any{"branch": "nonexistent"}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	res = decode[apiGitResult](t, rec)
	assert.False(t, res.Ok)

	// Invalid branch name is rejected at the validation layer.
	rec = do(t, s, http.MethodPost, "/api/git/checkout", map[string]any{"branch": "", "create": true}, "X-Project", "test")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/git/checkout", map[string]any{"branch": "-evil", "create": true}, "X-Project", "test")
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// Start point for create can be a commit.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("world\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "second"))
	hash, err := runGit(dir, "rev-parse", "HEAD")
	require.NoError(t, err)
	rec = do(t, s, http.MethodPost, "/api/git/checkout", map[string]any{"branch": "from-commit", "create": true, "start_point": hash}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	res = decode[apiGitResult](t, rec)
	assert.True(t, res.Ok)
}

func TestGitBranchesNotARepo(t *testing.T) {
	s, _ := newTestServer(t)
	rec := do(t, s, http.MethodGet, "/api/git/branches", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	branches := decode[apiGitBranchesResult](t, rec)
	assert.Empty(t, branches.Branches)
}

func TestGitScopedSubdir(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "a.md"), []byte("a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "initial"))

	// Changes inside and outside the scope.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "a.md"), []byte("a edited\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "new.txt"), []byte("n\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r edited\n"), 0o644))

	// The whole-repo view reports everything.
	rec := do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail := decode[apiGitDetail](t, rec)
	require.True(t, detail.IsRepo)
	assert.Len(t, detail.Unstaged, 2)
	assert.Len(t, detail.Untracked, 1)

	// The scoped view is limited to the directory and reports paths relative
	// to the directory itself (so later pathspec operations line up).
	rec = do(t, s, http.MethodGet, "/api/git/status?path=sub", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	require.True(t, detail.IsRepo)
	require.Len(t, detail.Unstaged, 1)
	assert.Equal(t, "a.md", detail.Unstaged[0].Path)
	assert.Contains(t, detail.Unstaged[0].Diff, "a edited")
	require.Len(t, detail.Untracked, 1)
	assert.Equal(t, "new.txt", detail.Untracked[0].Path)

	// Staging via the scoped endpoint uses the reported relative paths.
	rec = do(t, s, http.MethodPost, "/api/git/stage?path=sub", map[string]any{"paths": []string{"a.md", "new.txt"}}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	res := decode[apiGitResult](t, rec)
	assert.True(t, res.Ok)
	rec = do(t, s, http.MethodGet, "/api/git/status?path=sub", nil, "X-Project", "test")
	detail = decode[apiGitDetail](t, rec)
	require.Len(t, detail.Staged, 2)

	// Committing from a scoped view must not stage changes outside the dir.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r edited again\n"), 0o644))
	rec = do(t, s, http.MethodPost, "/api/git/commit?path=sub", map[string]any{"message": "scoped"}, "X-Project", "test")
	res = decode[apiGitResult](t, rec)
	assert.True(t, res.Ok)
	log, err := runGit(dir, "status", "--porcelain")
	require.NoError(t, err)
	assert.Contains(t, log, "root.txt") // root change still present and uncommitted
	assert.NotContains(t, log, "a.md")

	// Discarding an untracked scoped file removes it.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "throwaway.txt"), []byte("t\n"), 0o644))
	rec = do(t, s, http.MethodPost, "/api/git/discard?path=sub", map[string]any{"paths": []string{"throwaway.txt"}}, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	_, statErr := os.Stat(filepath.Join(dir, "sub", "throwaway.txt"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestGitScopedNestedRepo(t *testing.T) {
	s, app := newTestServer(t)
	root := app.Project.Path
	require.NoError(t, runGitErr(root, "init"))
	require.NoError(t, os.WriteFile(filepath.Join(root, "root.txt"), []byte("r\n"), 0o644))
	require.NoError(t, runGitErr(root, "add", "-A"))
	require.NoError(t, runGitErr(root, "commit", "-m", "outer"))

	// A subdirectory managed by its own repository.
	inner := filepath.Join(root, "vendor")
	require.NoError(t, os.MkdirAll(inner, 0o755))
	require.NoError(t, runGitErr(inner, "init"))
	require.NoError(t, runGitErr(inner, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(inner, "config", "user.name", "test"))
	require.NoError(t, os.WriteFile(filepath.Join(inner, "lib.txt"), []byte("lib\n"), 0o644))
	require.NoError(t, runGitErr(inner, "add", "-A"))
	require.NoError(t, runGitErr(inner, "commit", "-m", "inner init"))

	// The nested repo is a single untracked directory entry in the outer
	// repository; it is hidden from the working-tree list so it cannot be
	// staged or discarded as if it were a plain directory. The scoped view
	// targets the inner repository itself.
	rec := do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	outer := decode[apiGitDetail](t, rec)
	require.Len(t, outer.Untracked, 0)

	require.NoError(t, os.WriteFile(filepath.Join(inner, "lib.txt"), []byte("lib edited\n"), 0o644))
	rec = do(t, s, http.MethodGet, "/api/git/status?path=vendor", nil, "X-Project", "test")
	innerDetail := decode[apiGitDetail](t, rec)
	require.True(t, innerDetail.IsRepo)
	require.Len(t, innerDetail.Unstaged, 1)
	assert.Equal(t, "lib.txt", innerDetail.Unstaged[0].Path)

	// Commits land in the inner repository, not the outer one.
	rec = do(t, s, http.MethodPost, "/api/git/commit?path=vendor", map[string]any{"message": "inner change"}, "X-Project", "test")
	res := decode[apiGitResult](t, rec)
	assert.True(t, res.Ok)
	log, err := runGit(inner, "log", "--format=%s")
	require.NoError(t, err)
	assert.Contains(t, log, "inner change")
	outerLog, err := runGit(root, "log", "--format=%s")
	require.NoError(t, err)
	assert.NotContains(t, outerLog, "inner change")
}

func TestGitScopedLogAndCommitDiff(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))
	require.NoError(t, runGitErr(dir, "config", "user.email", "test@example.com"))
	require.NoError(t, runGitErr(dir, "config", "user.name", "test"))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "a.md"), []byte("a\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "root.txt"), []byte("r\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "first"))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "b.md"), []byte("b\n"), 0o644))
	require.NoError(t, runGitErr(dir, "add", "-A"))
	require.NoError(t, runGitErr(dir, "commit", "-m", "second"))

	hashes, err := runGit(dir, "log", "--format=%H")
	require.NoError(t, err)
	lines := nonEmptyLines(hashes)
	require.Len(t, lines, 2)

	// The scoped log is limited to commits touching the directory.
	rec := do(t, s, http.MethodGet, "/api/git/log?path=sub", nil, "X-Project", "test")
	log := decode[apiGitLogResult](t, rec)
	require.Len(t, log.Commits, 2)
	assert.Equal(t, "second", log.Commits[0].Subject)

	// The scoped commit diff is restricted to the directory.
	rec = do(t, s, http.MethodGet, "/api/git/diff?path=sub&ref="+lines[0], nil, "X-Project", "test")
	diff := decode[apiGitCommitDiffResult](t, rec)
	assert.Contains(t, diff.Diff, "sub/b.md")
	assert.NotContains(t, diff.Diff, "root.txt")
}

func TestGitScopeInvalid(t *testing.T) {
	s, app := newTestServer(t)
	dir := app.Project.Path
	require.NoError(t, runGitErr(dir, "init"))

	for _, scope := range []string{"../escape", "..", "/abs", "a/../b"} {
		rec := do(t, s, http.MethodGet, "/api/git/status?path="+scope, nil, "X-Project", "test")
		assert.Equal(t, http.StatusBadRequest, rec.Code, "scope %q should be rejected", scope)
	}
	assert.Equal(t, http.StatusBadRequest, do(t, s, http.MethodPost, "/api/git/commit?path=../escape", map[string]any{"message": "x"}, "X-Project", "test").Code)
}
