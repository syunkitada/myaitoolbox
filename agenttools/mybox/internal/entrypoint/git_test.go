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
	s, _ := newTestServer(t, false)
	rec := do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	detail := decode[apiGitDetail](t, rec)
	assert.False(t, detail.IsRepo)
}

func TestGitCommitCycle(t *testing.T) {
	s, app := newTestServer(t, false)
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

func TestGitCommitFallsBackToLocalIdentity(t *testing.T) {
	// Hide any ambient identity so the first commit would normally fail with
	// "Please tell me who you are"; the handler must fall back to a local
	// placeholder identity instead of surfacing the failure.
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	s, app := newTestServer(t, false)
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
	s, app := newTestServer(t, false)
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
	s2, _ := newTestServer(t, false)
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
	s, app := newTestServer(t, false)
	rec := do(t, s, http.MethodPost, "/api/git/init", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	detail := decode[apiGitDetail](t, rec)
	assert.True(t, detail.IsRepo)
	assert.NotEmpty(t, app.Project.Path)
	assert.DirExists(t, filepath.Join(app.Project.Path, ".git"))
}

func TestGitReadOnly(t *testing.T) {
	s, app := newTestServer(t, true)
	require.NoError(t, runGitErr(app.Project.Path, "init"))

	rec := do(t, s, http.MethodPost, "/api/git/commit", map[string]any{"message": "nope"}, "X-Project", "test")
	assert.Equal(t, http.StatusForbidden, rec.Code)

	rec = do(t, s, http.MethodPost, "/api/git/push", nil, "X-Project", "test")
	assert.Equal(t, http.StatusForbidden, rec.Code)

	rec = do(t, s, http.MethodGet, "/api/git/status", nil, "X-Project", "test")
	assert.Equal(t, http.StatusOK, rec.Code)
}
