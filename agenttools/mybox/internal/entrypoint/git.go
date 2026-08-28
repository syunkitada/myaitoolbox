package entrypoint

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// gitOpsMu serializes git commands so concurrent UI operations (for example a
// push racing a commit) do not corrupt the repository state.
var gitOpsMu sync.Mutex

// gitFile describes a single changed file in the detail response.
type gitFile struct {
	Path   string `json:"path"`
	Status string `json:"status"` // staged, unstaged, untracked
	Code   string `json:"code"`   // porcelain status code, e.g. M, A, D, ??,
	Diff   string `json:"diff"`
}

// gitDetail is the response of GET /api/git/status.
type gitDetail struct {
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

// runGitRaw executes git and returns the raw combined output without any
// whitespace trimming. It is used where every byte matters, such as parsing
// `git status --porcelain` whose leading space in column 1 is significant.
func runGitRaw(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runGit(dir string, args ...string) (string, error) {
	out, err := runGitRaw(dir, args...)
	return strings.TrimSpace(out), err
}

// registerGitRoutes mounts the git API handlers on an echo router. The base
// path is empty when the server runs at the root; it carries the base path
// (for example /mybox) when running behind a reverse proxy.
func (s *Server) registerGitRoutes(e *echo.Echo, basePath string) {
	wrap := func(method, path string, h http.HandlerFunc) {
		e.Add(method, basePath+path, echo.WrapHandler(h))
	}
	wrap(http.MethodGet, "/api/git/status", s.GetGitStatus)
	wrap(http.MethodPost, "/api/git/commit", s.PostGitCommit)
	wrap(http.MethodPost, "/api/git/pull", s.PostGitPull)
	wrap(http.MethodPost, "/api/git/push", s.PostGitPush)
	wrap(http.MethodPost, "/api/git/init", s.PostGitInit)
	wrap(http.MethodPost, "/api/git/stage", s.PostGitStage)
	wrap(http.MethodPost, "/api/git/unstage", s.PostGitUnstage)
	wrap(http.MethodPost, "/api/git/discard", s.PostGitDiscard)
}

func gitRepoDetail(dir string) gitDetail {
	d := gitDetail{}
	if !isGitDir(dir) {
		return d
	}
	d.IsRepo = true
	d.Branch, _ = runGit(dir, "branch", "--show-current")
	if d.Branch == "" {
		d.Branch, _ = runGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	}
	d.LastCommitMessage, _ = runGit(dir, "log", "-1", "--format=%s")
	if remotes, err := runGit(dir, "remote"); err == nil && remotes != "" {
		d.Remote = strings.SplitN(remotes, "\n", 2)[0]
	}
	if upstream, err := runGit(dir, "rev-parse", "--abbrev-ref", "@{upstream}"); err == nil && upstream != "" {
		if counts, err2 := runGit(dir, "rev-list", "--left-right", "--count", upstream+"...HEAD"); err2 == nil {
			parts := strings.Fields(counts)
			if len(parts) == 2 {
				d.Behind, _ = strconv.Atoi(parts[0])
				d.Ahead, _ = strconv.Atoi(parts[1])
			}
		}
	}
	porcelain, _ := runGitRaw(dir, "status", "--porcelain", "--untracked-files=all")
	d.Staged = []*gitFile{}
	d.Unstaged = []*gitFile{}
	d.Untracked = []*gitFile{}
	for _, line := range strings.Split(porcelain, "\n") {
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		path := strings.TrimLeft(line[2:], " ")
		// Renames are reported as "old -> new"; the new name is what the
		// diff command (and the file manager) cares about.
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		switch {
		case x == '?' && y == '?':
			d.Untracked = append(d.Untracked, &gitFile{
				Path: path, Status: "untracked", Code: "??", Diff: untrackedDiff(dir, path),
			})
		default:
			if x != ' ' && x != '?' {
				d.Staged = append(d.Staged, &gitFile{
					Path: path, Status: "staged", Code: string(x), Diff: gitFileDiff(dir, path, true),
				})
			}
			if y != ' ' && y != '?' {
				d.Unstaged = append(d.Unstaged, &gitFile{
					Path: path, Status: "unstaged", Code: string(y), Diff: gitFileDiff(dir, path, false),
				})
			}
		}
	}
	return d
}

func gitFileDiff(dir, path string, cached bool) string {
	args := []string{"diff"}
	if cached {
		args = append(args, "--cached")
	}
	args = append(args, "--", path)
	out, err := runGitRaw(dir, args...)
	if err != nil {
		return ""
	}
	return strings.TrimRight(out, "\r\n")
}

// untrackedDiff produces a unified diff for a new file by diffing it against
// /dev/null. Git exits with status 1 when files differ, which is expected. The
// path is passed relative to the repo root so the diff header shows the
// repository-relative name.
func untrackedDiff(dir, path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "diff", "--no-index", "/dev/null", path)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return ""
		}
	}
	return strings.TrimRight(string(out), "\n")
}

// gitResult is the response body of mutating git operations.
type gitResult struct {
	Ok     bool   `json:"ok"`
	Output string `json:"output,omitempty"`
}

func writeGitResult(w http.ResponseWriter, status int, result gitResult) {
	writeJSONResponse(w, status, result)
}

func (s *Server) GetGitStatus(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	detail := gitRepoDetail(app.Project.Path)
	gitOpsMu.Unlock()
	writeJSONResponse(w, http.StatusOK, detail)
}

type gitCommitRequest struct {
	Message string `json:"message"`
	// StagedOnly commits only what is already staged; otherwise all changes
	// are added before committing.
	StagedOnly bool `json:"staged_only"`
	// Amend rewrites the most recent commit instead of creating a new one.
	// An empty message then keeps the previous commit message.
	Amend bool `json:"amend"`
}

func (s *Server) PostGitCommit(w http.ResponseWriter, r *http.Request) {
	if !s.ensureWritable(w) {
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req gitCommitRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Message) == "" && !req.Amend {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("commit message is required")})
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	if !req.StagedOnly {
		if out, err := runGit(app.Project.Path, "add", "-A"); err != nil {
			writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: out})
			return
		}
	}
	args := []string{"commit"}
	if req.Amend {
		args = append(args, "--amend")
		if strings.TrimSpace(req.Message) == "" {
			args = append(args, "--no-edit")
		} else {
			args = append(args, "-m", req.Message)
		}
	} else {
		args = append(args, "-m", req.Message)
	}
	name, _ := runGit(app.Project.Path, "config", "user.name")
	email, _ := runGit(app.Project.Path, "config", "user.email")
	if name == "" || email == "" {
		// Fresh setups often have no git identity configured. Inject a local
		// placeholder instead of surfacing "Please tell me who you are" so the
		// first commit succeeds out of the box.
		args = append([]string{"-c", "user.name=mybox", "-c", "user.email=mybox@local"}, args...)
	}
	out, err := runGit(app.Project.Path, args...)
	if err != nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: out})
		return
	}
	writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: out})
}

func (s *Server) PostGitPull(w http.ResponseWriter, r *http.Request) {
	if !s.ensureWritable(w) {
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	out, err := runGit(app.Project.Path, "pull")
	if err != nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: out})
		return
	}
	writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: out})
}

func (s *Server) PostGitPush(w http.ResponseWriter, r *http.Request) {
	if !s.ensureWritable(w) {
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	out, err := runGit(app.Project.Path, "push")
	if err != nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: out})
		return
	}
	writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: out})
}

func (s *Server) PostGitInit(w http.ResponseWriter, r *http.Request) {
	if !s.ensureWritable(w) {
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	if _, err := runGit(app.Project.Path, "rev-parse", "--is-inside-work-tree"); err == nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: "already a git repository"})
		return
	}
	out, err := runGit(app.Project.Path, "init")
	if err != nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: out})
		return
	}
	writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: out})
}

type gitPathsRequest struct {
	Paths []string `json:"paths"`
}

func (s *Server) PostGitStage(w http.ResponseWriter, r *http.Request) {
	s.gitPathsOp(w, r, "stage")
}

func (s *Server) PostGitUnstage(w http.ResponseWriter, r *http.Request) {
	s.gitPathsOp(w, r, "unstage")
}

func (s *Server) PostGitDiscard(w http.ResponseWriter, r *http.Request) {
	s.gitPathsOp(w, r, "discard")
}

func (s *Server) gitPathsOp(w http.ResponseWriter, r *http.Request, op string) {
	if !s.ensureWritable(w) {
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req gitPathsRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if len(req.Paths) == 0 {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("no paths specified")})
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	var (
		opOut string
		opErr error
	)
	switch op {
	case "stage":
		opOut, opErr = runGit(app.Project.Path, append([]string{"add", "--"}, req.Paths...)...)
	case "unstage":
		// reset -q works for both newly added files and staged
		// modifications, including on a repository without commits yet
		// (where restore --staged fails with "could not resolve HEAD").
		opOut, opErr = runGit(app.Project.Path, append([]string{"reset", "-q", "--"}, req.Paths...)...)
	case "discard":
		opOut, opErr = discardPaths(app.Project.Path, req.Paths)
	}
	if opErr != nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: opOut})
		return
	}
	writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: opOut})
}

// untrackedPath reports whether path is untracked (status ??).
func untrackedPath(dir, path string) bool {
	out, _ := runGitRaw(dir, "status", "--porcelain", "--", path)
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "??") {
			return true
		}
	}
	return false
}

// discardPaths reverts modifications of tracked files and deletes untracked
// ones. Untracked files are not covered by git restore, so they are removed
// from the working tree directly.
func discardPaths(dir string, paths []string) (string, error) {
	tracked := make([]string, 0, len(paths))
	for _, p := range paths {
		if untrackedPath(dir, p) {
			if err := os.RemoveAll(filepath.Join(dir, p)); err != nil {
				return "", err
			}
			continue
		}
		tracked = append(tracked, p)
	}
	if len(tracked) == 0 {
		return "", nil
	}
	return runGit(dir, append([]string{"restore", "--"}, tracked...)...)
}
