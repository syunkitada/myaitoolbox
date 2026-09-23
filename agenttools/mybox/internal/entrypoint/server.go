package entrypoint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/syunkitada/myaitoolbox/mybox/internal/application"
	"github.com/syunkitada/myaitoolbox/mybox/internal/domain"
	"github.com/syunkitada/myaitoolbox/mybox/internal/entrypoint/api"
	"github.com/syunkitada/myaitoolbox/mybox/internal/webui"
)

var webDist = func() fs.FS {
	sub, err := fs.Sub(webui.FS, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}()

type Server struct {
	config         *domain.Config
	apps           map[string]*App
	projects       *application.ProjectUseCase
	mu             sync.RWMutex
	defaultProject string
	basePath       string
	herdrRun       herdrRunFunc
	terminals      *terminalHub
}

func NewServer(cfg *domain.Config, defaultProject string, basePath string) *Server {
	if defaultProject == "" {
		defaultProject = cfg.DefaultProject
	}
	basePath = normalizeBasePath(basePath)
	return &Server{
		config:         cfg,
		apps:           make(map[string]*App),
		projects:       NewProjectApp(),
		defaultProject: defaultProject,
		basePath:       basePath,
		terminals:      newTerminalHub(),
	}
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath != "" && !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return basePath
}

func (s *Server) getApp(r *http.Request) (*App, error) {
	project := r.Header.Get("X-Project")
	s.mu.RLock()
	if project == "" {
		project = s.defaultProject
	}
	app, ok := s.apps[project]
	s.mu.RUnlock()
	if ok {
		return app, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if app, ok := s.apps[project]; ok {
		return app, nil
	}
	app, err := NewApp(r.Context(), project)
	if err != nil {
		return nil, err
	}
	s.apps[project] = app
	return app, nil
}

func (s *Server) refreshDefaultProject(ctx context.Context) {
	cfg, err := s.projects.Config.Load(ctx)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.config = cfg
	s.defaultProject = cfg.DefaultProject
	s.mu.Unlock()
}

func (s *Server) Handler() http.Handler {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Use(middleware.Recover())

	apiHandler := api.HandlerWithOptions(s, api.StdHTTPServerOptions{BaseURL: s.basePath})
	rawFile := echo.WrapHandler(http.HandlerFunc(s.GetFileRaw))
	uploadFiles := echo.WrapHandler(http.HandlerFunc(s.UploadFiles))
	if s.basePath == "" {
		e.GET("/api/files/raw", rawFile)
		e.POST("/api/files/upload", uploadFiles)
		e.GET("/api/terminal", s.Terminal)
		e.DELETE("/api/terminal/destroy", s.DestroyTerminal)
		e.GET("/api/stats", echo.WrapHandler(http.HandlerFunc(s.GetStats)))
		s.registerGitRoutes(e, "")
		s.registerHerdrRoutes(e, "")
		e.Any("/api/*", echo.WrapHandler(apiHandler))
		e.Any("/api", echo.WrapHandler(apiHandler))
		e.GET("/*", s.handleIndex)
		return e
	}
	g := e.Group(s.basePath)
	g.GET("/api/files/raw", rawFile)
	g.POST("/api/files/upload", uploadFiles)
	g.GET("/api/terminal", s.Terminal)
	g.DELETE("/api/terminal/destroy", s.DestroyTerminal)
	g.GET("/api/stats", echo.WrapHandler(http.HandlerFunc(s.GetStats)))
	s.registerGitRoutes(e, s.basePath)
	s.registerHerdrRoutes(e, s.basePath)
	g.Any("/api/*", echo.WrapHandler(apiHandler))
	g.Any("/api", echo.WrapHandler(apiHandler))
	g.GET("", s.handleIndex)
	g.GET("/*", s.handleIndex)
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusFound, s.basePath+"/")
	})
	return e
}

func (s *Server) handleIndex(c echo.Context) error {
	path := c.Request().URL.Path
	if s.basePath != "" {
		path = strings.TrimPrefix(path, s.basePath)
	}
	path = strings.TrimPrefix(path, "/")
	// Vite emits relative asset paths (./assets/...), so a request from a
	// project route can arrive as /projects/{project}/.../assets/... . Only
	// resolve the SPA entrypoint and compiled assets here. Stripping arbitrary
	// path prefixes would make a project route ending in README.md resolve to
	// the documentation file embedded at webDist/README.md instead of serving
	// index.html.
	assetPath := path
	if path != "index.html" && !strings.HasPrefix(path, "assets/") {
		if i := strings.Index(path, "/assets/"); i >= 0 {
			assetPath = path[i+1:]
		} else {
			assetPath = ""
		}
	}
	var f fs.File
	err := fs.ErrNotExist
	if assetPath != "" {
		f, err = webDist.Open(assetPath)
	}
	if err == nil {
		info, statErr := f.Stat()
		if statErr == nil && !info.IsDir() {
			data, readErr := io.ReadAll(f)
			_ = f.Close()
			if readErr != nil {
				return readErr
			}
			ctype := mime.TypeByExtension(filepath.Ext(path))
			if ctype == "" {
				ctype = "application/octet-stream"
			}
			c.Response().Header().Set("Content-Type", ctype)
			_, _ = c.Response().Write(data)
			return nil
		}
		_ = f.Close()
	}
	return s.serveIndex(c)
}

func (s *Server) serveIndex(c echo.Context) error {
	data, readErr := fs.ReadFile(webDist, "index.html")
	if readErr != nil {
		return readErr
	}
	html := string(data)
	if s.basePath != "" {
		inject := fmt.Sprintf(
			"<base href=\"%s/\">\n<script>window.__MYBOX_BASE__=%q;</script>",
			s.basePath, s.basePath,
		)
		html = strings.Replace(html, "<head>", "<head>\n"+inject, 1)
	}
	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = c.Response().Write([]byte(html))
	return nil
}

func (s *Server) ListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.projects.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.Project, 0, len(projects))
	for _, p := range projects {
		out = append(out, api.Project{Name: p.Name, Path: p.Path})
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req api.CreateProjectRequest
	if !decodeBody(w, r, &req) {
		return
	}
	project, err := s.projects.Add(r.Context(), req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	s.mu.Lock()
	delete(s.apps, project.Name)
	s.mu.Unlock()
	s.refreshDefaultProject(r.Context())
	writeJSONResponse(w, http.StatusCreated, api.Project{Name: project.Name, Path: project.Path})
}

func (s *Server) DeleteProject(w http.ResponseWriter, r *http.Request, name string) {
	if err := s.projects.Remove(r.Context(), name); err != nil {
		writeError(w, err)
		return
	}
	s.mu.Lock()
	delete(s.apps, name)
	s.mu.Unlock()
	s.refreshDefaultProject(r.Context())
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ReorderProjects(w http.ResponseWriter, r *http.Request) {
	var req api.ReorderProjectsRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := s.projects.Reorder(r.Context(), req.Names); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetProjectPaths(w http.ResponseWriter, r *http.Request, params api.GetProjectPathsParams) {
	var prefix string
	if params.Prefix != nil {
		prefix = *params.Prefix
	}
	paths, err := s.projects.PathCandidates(r.Context(), prefix)
	if err != nil {
		writeError(w, err)
		return
	}
	if paths == nil {
		paths = []string{}
	}
	writeJSONResponse(w, http.StatusOK, paths)
}

func (s *Server) GetProjectGitStatus(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.projects.Config.Load(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	result := make(map[string]api.ProjectGitStatus, len(cfg.Projects))
	for _, p := range cfg.Projects {
		status := gitStatus(p.Path)
		result[p.Name] = status
	}
	writeJSONResponse(w, http.StatusOK, result)
}

func isGitDir(dir string) bool {
	return isInsideWorkTree(dir)
}

func gitStatus(dir string) api.ProjectGitStatus {
	if !isGitDir(dir) {
		return api.ProjectGitStatus{}
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return api.ProjectGitStatus{}
	}
	var staged, modified, untracked int
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(line) < 2 {
			continue
		}
		x, y := line[0], line[1]
		if x == '?' && y == '?' {
			untracked++
		} else {
			if x != ' ' && x != '?' {
				staged++
			}
			if y != ' ' && y != '?' {
				modified++
			}
		}
	}
	dirty := staged+modified+untracked > 0
	return api.ProjectGitStatus{Dirty: dirty, Modified: modified, Staged: staged, Untracked: untracked}
}

func (s *Server) GetMeta(w http.ResponseWriter, r *http.Request) {
	project := r.Header.Get("X-Project")
	if project == "" {
		cfg, err := s.projects.Config.Load(r.Context())
		if err != nil {
			writeError(w, err)
			return
		}
		projects := make([]string, 0, len(cfg.Projects))
		for _, p := range cfg.Projects {
			projects = append(projects, p.Name)
		}
		writeJSONResponse(w, http.StatusOK, api.Meta{
			Project:        "",
			Projects:       projects,
			DefaultProject: cfg.DefaultProject,
			Tags:           []string{},
			Favorites:      []string{},
			RecentFiles:    []string{},
		})
		return
	}
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	state, err := app.State.Get(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	cfgProjects, err := s.projects.List(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	projects := make([]string, 0, len(cfgProjects))
	for _, p := range cfgProjects {
		projects = append(projects, p.Name)
	}
	defaultProject, err := s.projects.Default(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	tags, err := s.collectTags(r.Context(), app)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.Meta{
		Project:        app.Project.Name,
		Projects:       projects,
		DefaultProject: defaultProject,
		Tags:           tags,
		Favorites:      state.Favorites,
		RecentFiles:    state.RecentFiles,
	})
}

func (s *Server) collectTags(ctx context.Context, app *App) ([]string, error) {
	set := map[string]struct{}{}
	tasks, err := app.Tasks.List(ctx, application.TaskFilter{})
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		for _, tag := range t.Tags {
			set[tag] = struct{}{}
		}
	}
	markdownTags, err := app.Files.MarkdownTags(ctx)
	if err != nil {
		return nil, err
	}
	for _, tag := range markdownTags {
		set[tag] = struct{}{}
	}
	tags := make([]string, 0, len(set))
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags, nil
}

func (s *Server) UpdateFavorite(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.UpdateFavoriteRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.State.ToggleFavorite(r.Context(), req.Path, req.Enabled); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) RecordRecent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.RecordRecentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.State.RecordRecent(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) DeleteRecent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.RecordRecentRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.State.RemoveRecent(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListTasks(w http.ResponseWriter, r *http.Request, params api.ListTasksParams) {
	project := r.Header.Get("X-Project")
	filter := application.TaskFilter{}
	if params.All != nil {
		filter.All = *params.All
	}
	if params.Status != nil {
		filter.Status = string(*params.Status)
	}
	if params.Tag != nil {
		filter.Tag = *params.Tag
	}

	// X-Project ヘッダーがない = プロジェクト未選択 → 全プロジェクトを横断
	if project == "" {
		tasks, err := s.listAllProjectsTasks(r.Context(), filter)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSONResponse(w, http.StatusOK, toAPITasks(tasks))
		return
	}

	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	tasks, err := app.Tasks.List(r.Context(), filter)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, toAPITasks(tasks))
}

func (s *Server) listAllProjectsTasks(ctx context.Context, filter application.TaskFilter) ([]domain.Task, error) {
	projects, err := s.projects.List(ctx)
	if err != nil {
		return nil, err
	}
	var all []domain.Task
	for _, p := range projects {
		app, appErr := s.getAppByProject(ctx, p.Name)
		if appErr != nil {
			continue
		}
		tasks, taskErr := app.Tasks.List(ctx, filter)
		if taskErr != nil {
			continue
		}
		all = append(all, tasks...)
	}
	return all, nil
}

func (s *Server) getAppByProject(ctx context.Context, project string) (*App, error) {
	s.mu.RLock()
	app, ok := s.apps[project]
	s.mu.RUnlock()
	if ok {
		return app, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if app, ok := s.apps[project]; ok {
		return app, nil
	}
	app, err := NewApp(ctx, project)
	if err != nil {
		return nil, err
	}
	s.apps[project] = app
	return app, nil
}

func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.CreateTaskRequest
	if !decodeBody(w, r, &req) {
		return
	}
	input := application.TaskInput{Name: req.Name}
	if req.Description != nil {
		input.Description = *req.Description
	}
	if req.AgentKind != nil {
		input.AgentKind = *req.AgentKind
	}
	if req.Status != nil {
		input.Status = string(*req.Status)
	}
	if req.Priority != nil {
		input.Priority = string(*req.Priority)
	}
	if req.Assignee != nil {
		input.Assignee = *req.Assignee
	}
	if req.Due != nil {
		input.Due = *req.Due
	}
	if req.Tags != nil {
		input.Tags = *req.Tags
	}
	task, err := app.Tasks.Create(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, toAPITask(*task))
}

func (s *Server) GetTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	task, err := app.Tasks.Show(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, toAPITask(*task))
}

func (s *Server) UpdateTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.UpdateTaskRequest
	if !decodeBody(w, r, &req) {
		return
	}
	patch := application.TaskPatch{}
	if req.Name != nil {
		patch.Name = req.Name
	}
	if req.Description != nil {
		patch.Description = req.Description
	}
	if req.AgentKind != nil {
		patch.AgentKind = req.AgentKind
	}
	if req.Status != nil {
		status := string(*req.Status)
		patch.Status = &status
	}
	if req.Priority != nil {
		priority := string(*req.Priority)
		patch.Priority = &priority
	}
	if req.Assignee != nil {
		patch.Assignee = req.Assignee
	}
	if req.Due != nil {
		patch.Due = req.Due
	}
	if req.Tags != nil {
		patch.Tags = req.Tags
	}
	task, err := app.Tasks.Patch(r.Context(), id, patch)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, toAPITask(*task))
}

func (s *Server) ArchiveTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := app.Tasks.Archive(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) DeleteTask(w http.ResponseWriter, r *http.Request, id string) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := app.Tasks.Delete(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) ListFiles(w http.ResponseWriter, r *http.Request, params api.ListFilesParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	showHidden := true
	if params.ShowHidden != nil {
		showHidden = *params.ShowHidden
	}
	var entries []domain.FileEntry
	if params.Path != nil {
		entries, err = app.Files.Children(r.Context(), *params.Path, showHidden)
	} else {
		entries, err = app.Files.Tree(r.Context(), showHidden)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	out := make([]api.FileEntry, 0, len(entries))
	for _, e := range entries {
		entry := api.FileEntry{Path: e.Path, Name: e.Name, Kind: api.FileEntryKind(e.Kind)}
		if e.Status != "" {
			entry.Status = &e.Status
		}
		if e.Executable {
			exec := true
			entry.Executable = &exec
		}
		out = append(out, entry)
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) ExecuteFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	res, err := app.Files.Execute(r.Context(), req.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	out := api.FileExecuteResult{Path: req.Path, ExitCode: res.ExitCode, Output: res.Output}
	if res.TimedOut {
		timedOut := true
		out.TimedOut = &timedOut
	}
	writeJSONResponse(w, http.StatusOK, out)
}

func (s *Server) GetFileContent(w http.ResponseWriter, r *http.Request, params api.GetFileContentParams) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	content, err := app.Files.Content(r.Context(), params.Path)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSONResponse(w, http.StatusOK, api.FileContent{Path: params.Path, Content: content})
}

// GetFileRaw streams a project file as raw bytes (used for image embeds in
// markdown, where the browser cannot send the X-Project header).
func (s *Server) GetFileRaw(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	path := q.Get("path")
	project := q.Get("project")
	var (
		app *App
		err error
	)
	if project != "" {
		app, err = s.getAppByProject(r.Context(), project)
	} else {
		app, err = s.getApp(r)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	data, err := app.Files.Raw(r.Context(), path)
	if err != nil {
		writeError(w, err)
		return
	}
	ctype := mime.TypeByExtension(filepath.Ext(path))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ctype)
	_, _ = w.Write(data)
}

func (s *Server) CreateFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Create(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CreateDir(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.CreateDir(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) SaveFileContent(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FileContent
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Save(r.Context(), req.Path, req.Content); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

const maxUploadSize = 100 << 20

// UploadFiles stores one or more local files in a project directory. The
// multipart endpoint is intentionally kept outside the generated JSON API
// because file uploads need multipart parsing rather than JSON decoding.
func (s *Server) UploadFiles(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, fmt.Errorf("%w: invalid multipart form", domain.ErrInvalidArgument))
		return
	}
	if r.MultipartForm != nil {
		defer func() {
			// MultipartForm cleanup happens after the response is determined;
			// a cleanup failure cannot be reported as a new HTTP error here.
			_ = r.MultipartForm.RemoveAll()
		}()
	}

	directory := r.FormValue("directory")
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, fmt.Errorf("%w: no files supplied", domain.ErrInvalidArgument))
		return
	}

	for _, header := range files {
		name := filepath.Base(strings.ReplaceAll(header.Filename, string(rune(92)), "/"))
		if name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) {
			writeError(w, fmt.Errorf("%w: invalid file name", domain.ErrInvalidArgument))
			return
		}
		path := name
		if directory != "" {
			path = directory + "/" + name
		}

		file, err := header.Open()
		if err != nil {
			writeError(w, err)
			return
		}
		data, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			writeError(w, readErr)
			return
		}
		if closeErr != nil {
			writeError(w, closeErr)
			return
		}
		if err := app.Files.SaveBytes(r.Context(), path, data); err != nil {
			writeError(w, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) MoveFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.MoveFileRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Move(r.Context(), req.OldPath, req.NewPath); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) CopyFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.MoveFileRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Copy(r.Context(), req.OldPath, req.NewPath); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) DeleteFile(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	var req api.FilePathRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if err := app.Files.Delete(r.Context(), req.Path); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) GetFileGitStatus(w http.ResponseWriter, r *http.Request) {
	app, err := s.getApp(r)
	if err != nil {
		writeError(w, err)
		return
	}
	result := fileGitStatus(app.Project.Path)
	writeJSONResponse(w, http.StatusOK, result)
}

func fileGitStatus(dir string) map[string]string {
	if !isGitDir(dir) {
		return map[string]string{}
	}
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return map[string]string{}
	}
	result := make(map[string]string)
	for _, line := range bytes.Split(out, []byte("\n")) {
		if len(line) < 3 {
			continue
		}
		x, y := line[0], line[1]
		path := string(bytes.TrimLeft(line[2:], " "))
		if x == '?' && y == '?' {
			result[path] = "untracked"
		} else if x == 'D' || y == 'D' {
			result[path] = "deleted"
		} else if x != ' ' && x != '?' {
			result[path] = "staged"
		} else if y != ' ' && y != '?' {
			result[path] = "modified"
		}
	}
	return result
}

type httpError struct {
	status int
	err    error
}

func (e *httpError) Error() string {
	return e.err.Error()
}

func (e *httpError) Unwrap() error {
	return e.err
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		writeError(w, fmt.Errorf("%w: invalid request body", domain.ErrInvalidArgument))
		return false
	}
	return true
}

func writeJSONResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	var he *httpError
	switch {
	case errors.As(err, &he):
		status = he.status
	case errors.Is(err, domain.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrInvalidArgument), errors.Is(err, domain.ErrInvalidPath):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrAlreadyExists):
		status = http.StatusConflict
	}
	writeJSONResponse(w, status, map[string]string{"error": err.Error()})
}

func toAPITasks(tasks []domain.Task) []api.Task {
	out := make([]api.Task, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, toAPITask(t))
	}
	return out
}

func toAPITask(t domain.Task) api.Task {
	return api.Task{
		Id:            t.ID,
		Title:         t.Title,
		Description:   strPtr(t.Description),
		AgentKind:     strPtr(t.AgentKind),
		Status:        api.TaskStatus(t.Status),
		Priority:      api.TaskPriority(t.Priority),
		Assignee:      strPtr(t.Assignee),
		Due:           strPtr(t.Due),
		PendingUntil:  strPtr(t.PendingUntil),
		PendingReason: strPtr(t.PendingReason),
		Tags:          &t.Tags,
		Project:       strPtr(t.Project),
		Created:       &t.Created,
		Body:          strPtr(t.Body),
		Archived:      &t.Archived,
	}
}

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
