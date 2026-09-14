package entrypoint

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
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
	wrap(http.MethodGet, "/api/git/log", s.GetGitLog)
	wrap(http.MethodGet, "/api/git/diff", s.GetGitCommitDiff)
	wrap(http.MethodGet, "/api/git/branches", s.GetGitBranches)
	wrap(http.MethodPost, "/api/git/checkout", s.PostGitCheckout)
	wrap(http.MethodPost, "/api/git/commit", s.PostGitCommit)
	wrap(http.MethodPost, "/api/git/pull", s.PostGitPull)
	wrap(http.MethodPost, "/api/git/push", s.PostGitPush)
	wrap(http.MethodPost, "/api/git/init", s.PostGitInit)
	wrap(http.MethodPost, "/api/git/stage", s.PostGitStage)
	wrap(http.MethodPost, "/api/git/unstage", s.PostGitUnstage)
	wrap(http.MethodPost, "/api/git/discard", s.PostGitDiscard)
}

// isInsideWorkTree reports whether dir lies inside a git work tree. This is
// true both for a repository root and for any subdirectory of a repository,
// so scoped views fall back to the enclosing repository.
func isInsideWorkTree(dir string) bool {
	out, err := runGit(dir, "rev-parse", "--is-inside-work-tree")
	return err == nil && strings.TrimSpace(out) == "true"
}

// repoRoot resolves the absolute root of the git repository containing dir.
// It returns "" when dir is not inside a work tree.
func repoRoot(dir string) string {
	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// gitScopeDir resolves a ?path= scope (a directory relative to the project
// root) into the absolute working directory used for git commands. An empty
// scope uses the project root itself.
func gitScopeDir(app *App, scope string) (string, error) {
	if scope == "" {
		return app.Project.Path, nil
	}
	if err := validateGitPath(scope); err != nil {
		return "", err
	}
	dir := filepath.Join(app.Project.Path, filepath.FromSlash(scope))
	if rel, err := filepath.Rel(app.Project.Path, dir); err != nil ||
		rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q", domain.ErrInvalidPath, scope)
	}
	return dir, nil
}

// gitScope returns the working directory implied by the request's optional
// ?path= query parameter.
func (s *Server) gitScope(r *http.Request, app *App) (string, error) {
	return gitScopeDir(app, r.URL.Query().Get("path"))
}

func gitRepoDetail(dir string) gitDetail {
	d := gitDetail{}
	if !isInsideWorkTree(dir) {
		return d
	}
	// git status reports paths relative to the repository root regardless of
	// the working directory, so filter the output down to the scope directory
	// (the working directory) when it is not the repository root itself.
	root := repoRoot(dir)
	scopePrefix := ""
	if root != "" && root != dir {
		if rel, err := filepath.Rel(root, dir); err == nil && rel != "." {
			scopePrefix = filepath.ToSlash(rel)
		}
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
		// The porcelain output covers the whole repository, so restrict it to
		// the scope directory reported by the working directory.
		if scopePrefix != "" && path != scopePrefix && !strings.HasPrefix(path, scopePrefix+"/") {
			continue
		}
		// Directories are reported as a single untracked entry when they are
		// nested repositories; skip them so the UI cannot stage or discard a
		// complete nested repo as if it were a file.
		if strings.HasSuffix(path, "/") {
			continue
		}
		// Rebase the repository-root-relative path onto the working directory.
		if root != "" {
			if rel, err := filepath.Rel(dir, filepath.Join(root, filepath.FromSlash(path))); err == nil {
				path = rel
			}
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

// gitLogEntry describes a single commit in the log response.
type gitLogEntry struct {
	Hash      string `json:"hash"`
	ShortHash string `json:"short_hash"`
	Author    string `json:"author"`
	Date      string `json:"date"`
	Subject   string `json:"subject"`
}

// gitLogResult is the response of GET /api/git/log.
type gitLogResult struct {
	Commits []*gitLogEntry `json:"commits"`
}

// gitCommitDiffResult is the response of GET /api/git/diff.
type gitCommitDiffResult struct {
	Diff string `json:"diff"`
}

// gitBranchInfo describes a single local branch.
type gitBranchInfo struct {
	Name     string `json:"name"`
	Current  bool   `json:"current"`
	Upstream string `json:"upstream,omitempty"`
}

// gitBranchesResult is the response of GET /api/git/branches.
type gitBranchesResult struct {
	Branches []*gitBranchInfo `json:"branches"`
}

// validateBranchName rejects ref-like names that could confuse git or be used
// to smuggle options. This mirrors git's own ref name rules.
func validateBranchName(name string) error {
	if name == "" {
		return errors.New("branch name is required")
	}
	if strings.HasPrefix(name, "-") {
		return errors.New("branch name must not start with '-'")
	}
	if strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("invalid branch name: %q", name)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return errors.New("branch name contains invalid characters")
		}
	}
	return nil
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
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	detail := gitRepoDetail(dir)
	gitOpsMu.Unlock()
	writeJSONResponse(w, http.StatusOK, detail)
}

// gitLogEntry represents a single commit line in the log format.
// Format: hash\nshort_hash\nauthor\ndate\nsubject (separated by Record Separator 0x1E)

func gitLogParseEntries(dir string, offset, count int) ([]*gitLogEntry, error) {
	args := []string{
		"log",
		"--format=%H%n%h%n%an%n%ai%n%s%x1e",
		"--skip=" + strconv.Itoa(offset),
		"-n", strconv.Itoa(count),
	}
	out, err := runGitRaw(dir, args...)
	if err != nil {
		// git log exits non-zero when there are no commits yet; treat as
		// an empty log rather than an error.
		if strings.Contains(out, "does not have any commits") || strings.TrimSpace(out) == "" {
			return []*gitLogEntry{}, nil
		}
		return nil, fmt.Errorf("git log failed: %s", strings.TrimSpace(out))
	}
	entries := []*gitLogEntry{}
	for _, block := range strings.Split(out, "\x1e") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		lines := strings.SplitN(block, "\n", 5)
		if len(lines) < 5 {
			continue
		}
		entries = append(entries, &gitLogEntry{
			Hash:      lines[0],
			ShortHash: lines[1],
			Author:    lines[2],
			Date:      lines[3],
			Subject:   lines[4],
		})
	}
	return entries, nil
}

func (s *Server) GetGitLog(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	if !isInsideWorkTree(dir) {
		writeJSONResponse(w, http.StatusOK, gitLogResult{Commits: []*gitLogEntry{}})
		return
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	count := 30
	if v := r.URL.Query().Get("count"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			count = n
		}
	}
	gitOpsMu.Lock()
	commits, logErr := gitLogParseEntries(dir, offset, count)
	gitOpsMu.Unlock()
	if logErr != nil {
		writeError(w, logErr)
		return
	}
	writeJSONResponse(w, http.StatusOK, gitLogResult{Commits: commits})
}

func (s *Server) GetGitCommitDiff(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	commitRef := r.URL.Query().Get("ref")
	if commitRef == "" {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("ref parameter is required")})
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()

	// Check if this is the root commit (no parent).
	isRoot := false
	if _, err := runGit(dir, "rev-parse", "--verify", commitRef+"^"); err != nil {
		isRoot = true
	}

	// Without a scope the diff covers the whole repo. When scoped to a
	// directory, git is run from that directory and `-- .` restricts the
	// change display to the directory (and any repository nested inside).
	scoped := r.URL.Query().Get("path") != "" && dir != app.Project.Path
	var out string
	if isRoot {
		args := []string{"show", commitRef}
		if scoped {
			args = append(args, "--", ".")
		}
		out, err = runGit(dir, args...)
	} else {
		args := []string{"diff", commitRef + "^.." + commitRef}
		if scoped {
			args = append(args, "--", ".")
		}
		out, err = runGit(dir, args...)
	}
	if err != nil {
		writeError(w, fmt.Errorf("failed to get diff for %s: %w", commitRef, err))
		return
	}
	writeJSONResponse(w, http.StatusOK, gitCommitDiffResult{Diff: out})
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

func (s *Server) GetGitBranches(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	if !isInsideWorkTree(dir) {
		writeJSONResponse(w, http.StatusOK, gitBranchesResult{Branches: []*gitBranchInfo{}})
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()

	// Obtain the current branch (empty when HEAD is detached).
	current, _ := runGit(dir, "symbolic-ref", "--short", "-q", "HEAD")

	// List local branches with upstream info.
	out, err := runGitRaw(dir,
		"for-each-ref",
		"--format=%(refname:short)%00%(upstream:short)",
		"refs/heads",
	)
	if err != nil && strings.TrimSpace(out) == "" {
		writeJSONResponse(w, http.StatusOK, gitBranchesResult{Branches: []*gitBranchInfo{}})
		return
	}

	branches := []*gitBranchInfo{}
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\x00", 2)
		name := strings.TrimSpace(parts[0])
		upstream := ""
		if len(parts) > 1 {
			upstream = strings.TrimSpace(parts[1])
		}
		branches = append(branches, &gitBranchInfo{
			Name:     name,
			Current:  name == current,
			Upstream: upstream,
		})
	}
	writeJSONResponse(w, http.StatusOK, gitBranchesResult{Branches: branches})
}

type gitCheckoutRequest struct {
	Branch     string `json:"branch"`
	Create     bool   `json:"create"`
	StartPoint string `json:"start_point,omitempty"`
}

func (s *Server) PostGitCheckout(w http.ResponseWriter, r *http.Request) {
	if !s.ensureWritable(w) {
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	var req gitCheckoutRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := validateBranchName(req.Branch); err != nil {
		writeError(w, &httpError{status: http.StatusBadRequest, err: err})
		return
	}
	if req.StartPoint != "" && req.StartPoint[0] == '-' {
		writeError(w, &httpError{status: http.StatusBadRequest, err: errors.New("start_point must not start with '-'")})
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	var args []string
	if req.Create {
		args = []string{"checkout", "-b", req.Branch}
		if req.StartPoint != "" {
			args = append(args, req.StartPoint)
		}
	} else {
		args = []string{"checkout", req.Branch}
	}
	out, err := runGit(dir, args...)
	if err != nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: out})
		return
	}
	writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: out})
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
	dir, err := s.gitScope(r, app)
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
		// `git add -A` from a subdirectory stages changes across the whole
		// repository, so a scoped commit limits staging to the scope dir with
		// `-- .` to only ever commit changes visible in the view.
		addArgs := []string{"add", "-A"}
		if dir != app.Project.Path {
			addArgs = append(addArgs, "--", ".")
		}
		if out, err := runGit(dir, addArgs...); err != nil {
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
	name, _ := runGit(dir, "config", "user.name")
	email, _ := runGit(dir, "config", "user.email")
	if name == "" || email == "" {
		// Fresh setups often have no git identity configured. Inject a local
		// placeholder instead of surfacing "Please tell me who you are" so the
		// first commit succeeds out of the box.
		args = append([]string{"-c", "user.name=mybox", "-c", "user.email=mybox@local"}, args...)
	}
	out, err := runGit(dir, args...)
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
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	out, err := runGit(dir, "pull")
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
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	out, err := runGit(dir, "push")
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
	dir, err := s.gitScope(r, app)
	if err != nil {
		writeError(w, err)
		return
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	if _, err := runGit(dir, "rev-parse", "--is-inside-work-tree"); err == nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: "already a git repository"})
		return
	}
	out, err := runGit(dir, "init")
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
	dir, err := s.gitScope(r, app)
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
	for _, p := range req.Paths {
		if err := validateGitPath(p); err != nil {
			writeError(w, err)
			return
		}
	}
	gitOpsMu.Lock()
	defer gitOpsMu.Unlock()
	var (
		opOut string
		opErr error
	)
	switch op {
	case "stage":
		opOut, opErr = runGit(dir, append([]string{"--literal-pathspecs", "add", "--"}, req.Paths...)...)
	case "unstage":
		// reset -q works for both newly added files and staged
		// modifications, including on a repository without commits yet
		// (where restore --staged fails with "could not resolve HEAD").
		opOut, opErr = runGit(dir, append([]string{"--literal-pathspecs", "reset", "-q", "--"}, req.Paths...)...)
	case "discard":
		opOut, opErr = discardPaths(dir, req.Paths)
	}
	if opErr != nil {
		writeGitResult(w, http.StatusOK, gitResult{Ok: false, Output: opOut})
		return
	}
	writeGitResult(w, http.StatusOK, gitResult{Ok: true, Output: opOut})
}

// validateGitPath rejects paths that could escape the project tree or smuggle
// pathspec magic (e.g. :(exclude)…), matching the validation applied to every
// other file-oriented endpoint. Git runs with --literal-pathspecs as a second
// line of defense so the paths are always treated as literal file paths.
func validateGitPath(p string) error {
	if p == "" || p == "." || p == ".." ||
		strings.HasPrefix(p, "/") || strings.HasPrefix(p, ":") ||
		strings.Contains(p, "..") ||
		strings.ContainsAny(p, `\`) {
		return fmt.Errorf("%w: %q", domain.ErrInvalidPath, p)
	}
	return nil
}

// untrackedPath reports whether path is untracked (status ??).
func untrackedPath(dir, path string) bool {
	out, _ := runGitRaw(dir, "--literal-pathspecs", "status", "--porcelain", "--", path)
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
	return runGit(dir, append([]string{"--literal-pathspecs", "restore", "--"}, tracked...)...)
}
